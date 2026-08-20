package gateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"alzette/internal/platform"
)

type requestFinisher func(string, int, string, string, string, string, platform.TokenUsage) bool
type attemptFinisher func(string, string, string, int) error

type streamSink struct {
	writer    http.ResponseWriter
	flusher   http.Flusher
	committed bool
}

func (sink *streamSink) write(body []byte) error {
	if len(body) == 0 {
		return nil
	}
	if !sink.committed {
		sink.writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		sink.writer.Header().Set("Cache-Control", "no-store")
		sink.writer.Header().Set("X-Accel-Buffering", "no")
		// Treat the first attempted downstream write as the commit boundary.
		// This is conservative if the writer fails before reporting bytes and
		// guarantees that such a request is never transparently replayed.
		sink.committed = true
	}
	written, err := sink.writer.Write(body)
	if err != nil {
		return err
	}
	if written != len(body) {
		return io.ErrShortWrite
	}
	sink.flusher.Flush()
	return nil
}

func (g *Gateway) serveStreaming(w http.ResponseWriter, r *http.Request, requestID, publicModel string, protocol wireProtocol, route platform.Route, secret string, upstreamBody []byte, finish requestFinisher) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		if !finish("failed", http.StatusInternalServerError, "streaming_unavailable", "", "", "unknown", platform.TokenUsage{}) {
			writeProtocolError(w, protocol, http.StatusServiceUnavailable, "ledger_unavailable", "api_error", "request result could not be recorded", requestID)
			return
		}
		writeProtocolError(w, protocol, http.StatusInternalServerError, "streaming_unavailable", "api_error", "response streaming is unavailable", requestID)
		return
	}
	sink := &streamSink{writer: w, flusher: flusher}
	encoder := newProtocolStreamEncoder(protocol, requestID, publicModel, g.clock().UTC())
	var terminal attemptResult
	for attemptNumber := 1; attemptNumber <= route.Target.MaxAttempts; attemptNumber++ {
		terminal = g.performStreamingAttempt(r.Context(), requestID, route, secret, upstreamBody, attemptNumber, sink, encoder)
		if terminal.success || sink.committed || !terminal.retryable || attemptNumber == route.Target.MaxAttempts {
			break
		}
		delay := terminal.retryAfter
		if !terminal.retryAfterSet {
			delay = g.retryBaseDelay * time.Duration(1<<(attemptNumber-1))
		}
		if delay > g.maxRetryDelay {
			delay = g.maxRetryDelay
		}
		if err := wait(r.Context(), delay); err != nil {
			terminal = attemptResult{class: "client_cancelled", clientStatus: 499, message: "request was cancelled"}
			break
		}
	}
	if terminal.success {
		if !finish("succeeded", http.StatusOK, "", terminal.model, terminal.providerID, terminal.finality, terminal.usage) {
			// The response may already be committed. Ending the stream without
			// synthesising a success or a provider error is the only safe action.
			if !sink.committed {
				writeProtocolError(w, protocol, http.StatusServiceUnavailable, "ledger_unavailable", "api_error", "request result could not be recorded", requestID)
			}
			return
		}
		healthContext, cancel := detachedContext()
		_ = g.store.UpdateTargetHealth(healthContext, route.Target.ID, "operational", g.clock().UTC(), true)
		cancel()
		return
	}
	status := "failed"
	if terminal.class == "client_cancelled" {
		status = "cancelled"
	}
	if terminal.clientStatus == 0 {
		terminal.clientStatus = http.StatusBadGateway
	}
	if terminal.message == "" {
		terminal.message = "upstream inference failed"
	}
	// Failed and cancelled requests intentionally persist no token values. A
	// completed stream with terminal usage is metered above; interrupted or
	// missing usage remains unknown rather than being guessed.
	if !finish(status, terminal.clientStatus, terminal.class, "", terminal.providerID, "unknown", platform.TokenUsage{}) {
		if !sink.committed {
			writeProtocolError(w, protocol, http.StatusServiceUnavailable, "ledger_unavailable", "api_error", "request result could not be recorded", requestID)
		}
		return
	}
	if healthStatus := failureHealthStatus(terminal.class); healthStatus != "" {
		healthContext, cancel := detachedContext()
		_ = g.store.UpdateTargetHealth(healthContext, route.Target.ID, healthStatus, g.clock().UTC(), false)
		cancel()
	}
	if sink.committed {
		return
	}
	if terminal.retryAfterHeader != "" {
		w.Header().Set("Retry-After", terminal.retryAfterHeader)
	}
	writeProtocolError(w, protocol, terminal.clientStatus, terminal.class, "upstream_error", terminal.message, requestID)
}

func (g *Gateway) performStreamingAttempt(parent context.Context, requestID string, route platform.Route, secret string, body []byte, number int, sink *streamSink, encoder protocolStreamEncoder) attemptResult {
	// Bifrost v1.7.13's direct fasthttp stream cancellation path races while
	// releasing its request stream. Keep Alzette's existing net/http SSE path
	// until the pinned dependency passes this package's cancellation race gate.
	attemptID, err := g.newID("att")
	if err != nil {
		return attemptResult{class: "ledger_error", clientStatus: http.StatusInternalServerError, message: "request attempt could not be initialised"}
	}
	started := g.clock().UTC()
	if err := g.store.CreateProviderAttempt(parent, platform.AttemptStart{ID: attemptID, InferenceRequestID: requestID, TargetID: route.Target.ID, AttemptNumber: number, StartedAt: started}); err != nil {
		return attemptResult{class: "ledger_error", clientStatus: http.StatusServiceUnavailable, message: "request attempt could not be recorded"}
	}
	finish := func(status, class, providerID string, providerStatus int) error {
		finishContext, cancel := detachedContext()
		defer cancel()
		return g.store.CompleteProviderAttempt(finishContext, platform.AttemptFinish{ID: attemptID, CompletedAt: g.clock().UTC(), Status: status, ProviderHTTPStatus: providerStatus, ErrorClass: class, Duration: elapsed(started, g.clock().UTC()), ProviderRequestID: providerID})
	}
	ledgerFailure := func() attemptResult {
		return attemptResult{class: "ledger_error", clientStatus: http.StatusServiceUnavailable, message: "request attempt result could not be recorded"}
	}
	attemptContext, cancel := context.WithTimeout(parent, route.Target.Timeout)
	defer cancel()
	upstreamURL, err := targetEndpoint(route.Target.BaseURL, g.allowInsecureTargets)
	if err != nil {
		if finish("failed", "target_configuration", "", 0) != nil {
			return ledgerFailure()
		}
		return attemptResult{class: "target_configuration", clientStatus: http.StatusBadGateway, message: "configured inference target is invalid"}
	}
	request, err := http.NewRequestWithContext(attemptContext, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		if finish("failed", "target_configuration", "", 0) != nil {
			return ledgerFailure()
		}
		return attemptResult{class: "target_configuration", clientStatus: http.StatusBadGateway, message: "configured inference target is invalid"}
	}
	request.Header.Set("Authorization", "Bearer "+secret)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("X-Alzette-Request-ID", requestID)
	response, err := g.client.Do(request)
	if err != nil {
		return g.finishStreamingTransportError(parent, attemptContext, finish, "", 0)
	}
	defer response.Body.Close()
	providerID := safeProviderID(response.Header.Get("X-Generation-Id"))
	if response.StatusCode != http.StatusOK {
		responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, g.maxResponseBytes+1))
		if readErr != nil {
			return g.finishStreamingTransportError(parent, attemptContext, finish, providerID, response.StatusCode)
		}
		if int64(len(responseBody)) > g.maxResponseBytes {
			if finish("failed", "upstream_response_too_large", providerID, response.StatusCode) != nil {
				return ledgerFailure()
			}
			return attemptResult{class: "upstream_response_too_large", clientStatus: http.StatusBadGateway, message: "upstream inference response exceeded the configured limit", providerID: providerID}
		}
		result := classifyProviderStatus(response.StatusCode)
		result.providerID = providerID
		if response.StatusCode == http.StatusTooManyRequests || response.StatusCode == http.StatusServiceUnavailable {
			result.retryAfter, result.retryAfterSet, result.retryAfterHeader = parseRetryAfter(response.Header.Get("Retry-After"), g.clock().UTC())
		}
		if finish("failed", result.class, providerID, response.StatusCode) != nil {
			return ledgerFailure()
		}
		return result
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "text/event-stream" {
		if finish("failed", "invalid_upstream_response", providerID, response.StatusCode) != nil {
			return ledgerFailure()
		}
		return attemptResult{class: "invalid_upstream_response", clientStatus: http.StatusBadGateway, message: "upstream inference returned an invalid streaming response", providerID: providerID}
	}

	reader := bufio.NewReader(io.LimitReader(response.Body, g.maxResponseBytes+1))
	state := streamState{providerID: providerID, finality: "unknown"}
	var pending bytes.Buffer
	var total int64
	for {
		frame, readErr := readSSEFrame(reader)
		total += int64(len(frame.raw))
		if total > g.maxResponseBytes {
			if finish("failed", "upstream_response_too_large", state.providerID, response.StatusCode) != nil {
				return ledgerFailure()
			}
			return attemptResult{class: "upstream_response_too_large", clientStatus: http.StatusBadGateway, message: "upstream inference response exceeded the configured limit", providerID: state.providerID}
		}
		if readErr != nil {
			if parent.Err() != nil {
				if finish("cancelled", "client_cancelled", state.providerID, response.StatusCode) != nil {
					return ledgerFailure()
				}
				return attemptResult{class: "client_cancelled", clientStatus: 499, message: "request was cancelled", providerID: state.providerID}
			}
			if errors.Is(attemptContext.Err(), context.DeadlineExceeded) {
				if finish("failed", "upstream_timeout", state.providerID, response.StatusCode) != nil {
					return ledgerFailure()
				}
				return attemptResult{retryable: !sink.committed, class: "upstream_timeout", clientStatus: http.StatusGatewayTimeout, message: "upstream inference timed out", providerID: state.providerID}
			}
			if !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
				if finish("failed", "invalid_upstream_response", state.providerID, response.StatusCode) != nil {
					return ledgerFailure()
				}
				return attemptResult{class: "invalid_upstream_response", clientStatus: http.StatusBadGateway, message: "upstream inference returned an invalid streaming response", providerID: state.providerID}
			}
			if finish("failed", "upstream_transport", state.providerID, response.StatusCode) != nil {
				return ledgerFailure()
			}
			return attemptResult{retryable: !sink.committed, class: "upstream_transport", clientStatus: http.StatusBadGateway, message: "upstream inference stream ended before completion", providerID: state.providerID}
		}
		if !frame.hasData {
			if encoder == nil {
				_, _ = pending.Write(frame.raw)
			}
			continue
		}
		if frame.data == "[DONE]" {
			if !state.sawChoice || !state.sawFinish {
				if finish("failed", "invalid_upstream_response", state.providerID, response.StatusCode) != nil {
					return ledgerFailure()
				}
				return attemptResult{class: "invalid_upstream_response", clientStatus: http.StatusBadGateway, message: "upstream inference stream ended without a terminal choice", providerID: state.providerID}
			}
			if encoder == nil {
				pending.Write(frame.raw)
			} else {
				translated, translateErr := encoder.Finish(state)
				if translateErr != nil {
					if finish("failed", "invalid_upstream_response", state.providerID, response.StatusCode) != nil {
						return ledgerFailure()
					}
					return attemptResult{class: "invalid_upstream_response", clientStatus: http.StatusBadGateway, message: "upstream inference stream could not be represented in the requested API", providerID: state.providerID}
				}
				pending.Write(translated)
			}
			if err := sink.write(pending.Bytes()); err != nil {
				if finish("cancelled", "client_cancelled", state.providerID, response.StatusCode) != nil {
					return ledgerFailure()
				}
				return attemptResult{class: "client_cancelled", clientStatus: 499, message: "request was cancelled", providerID: state.providerID}
			}
			if finish("succeeded", "", state.providerID, response.StatusCode) != nil {
				return ledgerFailure()
			}
			return attemptResult{success: true, model: state.model, providerID: state.providerID, usage: state.usage, finality: state.finality}
		}
		if err := state.consume(frame.data); err != nil {
			if finish("failed", "invalid_upstream_response", state.providerID, response.StatusCode) != nil {
				return ledgerFailure()
			}
			return attemptResult{class: "invalid_upstream_response", clientStatus: http.StatusBadGateway, message: "upstream inference returned an invalid streaming response", providerID: state.providerID}
		}
		if encoder == nil {
			pending.Write(frame.raw)
		} else {
			translated, translateErr := encoder.Encode(frame.data)
			if translateErr != nil {
				if finish("failed", "invalid_upstream_response", state.providerID, response.StatusCode) != nil {
					return ledgerFailure()
				}
				return attemptResult{class: "invalid_upstream_response", clientStatus: http.StatusBadGateway, message: "upstream inference stream could not be represented in the requested API", providerID: state.providerID}
			}
			pending.Write(translated)
		}
		if err := sink.write(pending.Bytes()); err != nil {
			if finish("cancelled", "client_cancelled", state.providerID, response.StatusCode) != nil {
				return ledgerFailure()
			}
			return attemptResult{class: "client_cancelled", clientStatus: 499, message: "request was cancelled", providerID: state.providerID}
		}
		pending.Reset()
	}
}

func (g *Gateway) finishStreamingTransportError(parent, attemptContext context.Context, finishAttempt attemptFinisher, providerID string, providerStatus int) attemptResult {
	if parent.Err() != nil {
		if finishAttempt("cancelled", "client_cancelled", providerID, providerStatus) != nil {
			return attemptResult{class: "ledger_error", clientStatus: http.StatusServiceUnavailable, message: "request attempt result could not be recorded"}
		}
		return attemptResult{class: "client_cancelled", clientStatus: 499, message: "request was cancelled", providerID: providerID}
	}
	if errors.Is(attemptContext.Err(), context.DeadlineExceeded) {
		if finishAttempt("failed", "upstream_timeout", providerID, providerStatus) != nil {
			return attemptResult{class: "ledger_error", clientStatus: http.StatusServiceUnavailable, message: "request attempt result could not be recorded"}
		}
		return attemptResult{retryable: true, class: "upstream_timeout", clientStatus: http.StatusGatewayTimeout, message: "upstream inference timed out", providerID: providerID}
	}
	if finishAttempt("failed", "upstream_transport", providerID, providerStatus) != nil {
		return attemptResult{class: "ledger_error", clientStatus: http.StatusServiceUnavailable, message: "request attempt result could not be recorded"}
	}
	return attemptResult{retryable: true, class: "upstream_transport", clientStatus: http.StatusBadGateway, message: "upstream inference connection failed", providerID: providerID}
}

type sseFrame struct {
	raw     []byte
	data    string
	hasData bool
}

func readSSEFrame(reader *bufio.Reader) (sseFrame, error) {
	var raw bytes.Buffer
	var data []string
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			_, _ = raw.WriteString(line)
		}
		if err != nil {
			return sseFrame{raw: raw.Bytes()}, err
		}
		value := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if value == "" {
			return sseFrame{raw: raw.Bytes(), data: strings.Join(data, "\n"), hasData: len(data) != 0}, nil
		}
		switch {
		case strings.HasPrefix(value, ":"):
			continue
		case strings.HasPrefix(value, "data:"):
			part := strings.TrimPrefix(value, "data:")
			if strings.HasPrefix(part, " ") {
				part = part[1:]
			}
			data = append(data, part)
		default:
			return sseFrame{raw: raw.Bytes()}, fmt.Errorf("unsupported SSE field")
		}
	}
}

type streamState struct {
	model, providerID string
	usage             platform.TokenUsage
	finality          string
	sawChoice         bool
	sawFinish         bool
}

type providerStreamChunk struct {
	ID      string          `json:"id"`
	Model   string          `json:"model"`
	Choices json.RawMessage `json:"choices"`
	Usage   *providerUsage  `json:"usage"`
	Error   json.RawMessage `json:"error"`
}

type providerStreamChoice struct {
	Index        int             `json:"index"`
	Delta        json.RawMessage `json:"delta"`
	FinishReason json.RawMessage `json:"finish_reason"`
}

type providerStreamDelta struct {
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	ToolCalls json.RawMessage `json:"tool_calls"`
}

type providerToolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function *struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func (state *streamState) consume(data string) error {
	var chunk providerStreamChunk
	if err := decodeJSONValue([]byte(data), &chunk); err != nil {
		return err
	}
	if len(chunk.Error) != 0 && !bytes.Equal(bytes.TrimSpace(chunk.Error), []byte("null")) {
		return fmt.Errorf("provider returned a streaming error")
	}
	if safe := safeProviderID(chunk.ID); safe != "" {
		state.providerID = safe
	}
	if safe := safeModel(chunk.Model); safe != "" {
		state.model = safe
	}
	if chunk.Usage != nil {
		if !state.sawChoice {
			return fmt.Errorf("usage preceded stream choices")
		}
		usage, finality, err := parseProviderUsage(chunk.Usage)
		if err != nil {
			return err
		}
		state.usage, state.finality = usage, finality
	}
	if len(chunk.Choices) == 0 {
		if chunk.Usage == nil {
			return fmt.Errorf("stream chunk has neither choices nor usage")
		}
		return nil
	}
	var choices []providerStreamChoice
	if err := json.Unmarshal(chunk.Choices, &choices); err != nil || len(choices) > maxMessages {
		return fmt.Errorf("invalid stream choices")
	}
	if len(choices) == 0 {
		if chunk.Usage == nil {
			return fmt.Errorf("empty stream choices without usage")
		}
		return nil
	}
	state.sawChoice = true
	for _, choice := range choices {
		if len(choice.Delta) != 0 && !bytes.Equal(bytes.TrimSpace(choice.Delta), []byte("null")) {
			if err := validateProviderStreamDelta(choice.Delta); err != nil {
				return err
			}
		}
		if len(choice.FinishReason) != 0 && !bytes.Equal(bytes.TrimSpace(choice.FinishReason), []byte("null")) {
			var reason string
			if err := json.Unmarshal(choice.FinishReason, &reason); err != nil || (reason != "stop" && reason != "length" && reason != "tool_calls") {
				return fmt.Errorf("unsupported stream finish reason")
			}
			state.sawFinish = true
		}
	}
	return nil
}

func validateProviderStreamDelta(raw json.RawMessage) error {
	var delta providerStreamDelta
	if err := json.Unmarshal(raw, &delta); err != nil {
		return err
	}
	if delta.Role != "" && delta.Role != "assistant" {
		return fmt.Errorf("unsupported stream delta role")
	}
	if len(delta.Content) != 0 && !bytes.Equal(bytes.TrimSpace(delta.Content), []byte("null")) {
		var content string
		if err := json.Unmarshal(delta.Content, &content); err != nil {
			return fmt.Errorf("stream delta content is not text")
		}
	}
	if len(delta.ToolCalls) == 0 || bytes.Equal(bytes.TrimSpace(delta.ToolCalls), []byte("null")) {
		return nil
	}
	var calls []providerToolCallDelta
	if err := json.Unmarshal(delta.ToolCalls, &calls); err != nil || len(calls) == 0 || len(calls) > maxToolCalls {
		return fmt.Errorf("invalid stream tool call deltas")
	}
	for _, call := range calls {
		if call.Index < 0 || (call.ID != "" && !validToolCallID(call.ID)) || (call.Type != "" && call.Type != "function") {
			return fmt.Errorf("unsupported stream tool call delta")
		}
		if call.Function != nil && call.Function.Name != "" && !validToolName(call.Function.Name) {
			return fmt.Errorf("invalid stream function name")
		}
	}
	return nil
}
