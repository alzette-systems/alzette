package gateway

// This bounded Anthropic Messages adapter follows the content-block and
// stop-reason mappings used by Maxim Bifrost's Apache-2.0 Anthropic adapter.
// It intentionally excludes provider-specific hosted tools, prompt caching,
// files, images, and computer use until Alzette can preserve those semantics
// through its canonical execution and ledger pipeline.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type anthropicRequest struct {
	Model         string             `json:"model"`
	MaxTokens     int                `json:"max_tokens"`
	Messages      []anthropicMessage `json:"messages"`
	System        json.RawMessage    `json:"system,omitempty"`
	Stream        *bool              `json:"stream,omitempty"`
	Temperature   *float64           `json:"temperature,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
	TopK          *int               `json:"top_k,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	Tools         []anthropicTool    `json:"tools,omitempty"`
	ToolChoice    json.RawMessage    `json:"tool_choice,omitempty"`
	Metadata      json.RawMessage    `json:"metadata,omitempty"`
	Thinking      json.RawMessage    `json:"thinking,omitempty"`
	ServiceTier   string             `json:"service_tier,omitempty"`
}

type anthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type anthropicContentBlock struct {
	Type         string          `json:"type"`
	Text         string          `json:"text,omitempty"`
	ID           string          `json:"id,omitempty"`
	Name         string          `json:"name,omitempty"`
	Input        json.RawMessage `json:"input,omitempty"`
	ToolUseID    string          `json:"tool_use_id,omitempty"`
	Content      json.RawMessage `json:"content,omitempty"`
	IsError      *bool           `json:"is_error,omitempty"`
	CacheControl json.RawMessage `json:"cache_control,omitempty"`
}

type anthropicTool struct {
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	InputSchema  json.RawMessage `json:"input_schema"`
	CacheControl json.RawMessage `json:"cache_control,omitempty"`
}

func decodeAnthropicRequest(data []byte) (ChatRequest, error) {
	var input anthropicRequest
	if err := decodeStrict(data, &input); err != nil {
		if strings.Contains(err.Error(), "json: unknown field") {
			return ChatRequest{}, unsupportedProtocolField("Anthropic Messages", strings.TrimPrefix(strings.TrimSpace(err.Error()), "json: unknown field "))
		}
		return ChatRequest{}, invalidProtocolRequest("Anthropic Messages", "is not valid JSON")
	}
	if input.MaxTokens < 1 {
		return ChatRequest{}, invalidProtocolRequest("Anthropic Messages", "requires max_tokens")
	}
	if input.TopK != nil {
		return ChatRequest{}, unsupportedProtocolField("Anthropic Messages", "top_k")
	}
	if len(bytes.TrimSpace(input.Thinking)) != 0 && !bytes.Equal(bytes.TrimSpace(input.Thinking), []byte("null")) {
		return ChatRequest{}, unsupportedProtocolField("Anthropic Messages", "thinking")
	}
	if input.ServiceTier != "" && input.ServiceTier != "auto" {
		return ChatRequest{}, unsupportedProtocolField("Anthropic Messages", "service_tier")
	}
	if !anthropicMetadataSafe(input.Metadata) {
		return ChatRequest{}, invalidProtocolRequest("Anthropic Messages", "metadata is invalid")
	}

	messages := make([]Message, 0, len(input.Messages)+1)
	if len(bytes.TrimSpace(input.System)) != 0 && !bytes.Equal(bytes.TrimSpace(input.System), []byte("null")) {
		system, err := anthropicTextBlocks(input.System)
		if err != nil || len(system) == 0 {
			return ChatRequest{}, invalidProtocolRequest("Anthropic Messages", "system must contain text blocks")
		}
		messages = append(messages, Message{Role: "system", Content: joinTextParts(system)})
	}
	for _, message := range input.Messages {
		converted, err := anthropicMessageToChat(message)
		if err != nil {
			return ChatRequest{}, err
		}
		messages = append(messages, converted...)
	}
	tools := make([]Tool, 0, len(input.Tools))
	for _, value := range input.Tools {
		if hasNonNull(value.CacheControl) {
			return ChatRequest{}, unsupportedProtocolField("Anthropic Messages", "tools.cache_control")
		}
		tools = append(tools, Tool{Type: "function", Function: ToolDefinition{Name: value.Name, Description: value.Description, Parameters: value.InputSchema}})
	}
	choice, err := anthropicToolChoice(input.ToolChoice)
	if err != nil {
		return ChatRequest{}, err
	}
	var stop json.RawMessage
	if len(input.StopSequences) != 0 {
		stop, _ = json.Marshal(input.StopSequences)
	}
	maxTokens := input.MaxTokens
	request := ChatRequest{
		Model: input.Model, Messages: messages, Stream: input.Stream,
		Temperature: input.Temperature, TopP: input.TopP, MaxTokens: &maxTokens,
		Stop: stop, Tools: tools, ToolChoice: choice,
	}
	if len(bytes.TrimSpace(input.ToolChoice)) != 0 && !bytes.Equal(bytes.TrimSpace(input.ToolChoice), []byte("null")) {
		var selected struct {
			DisableParallelToolUse *bool `json:"disable_parallel_tool_use,omitempty"`
		}
		if decodeStrict(input.ToolChoice, &selected) == nil && selected.DisableParallelToolUse != nil {
			parallel := !*selected.DisableParallelToolUse
			request.ParallelToolCalls = &parallel
		}
	}
	if request.streaming() {
		include := true
		request.StreamOptions = &StreamOptions{IncludeUsage: &include}
	}
	return request, nil
}

func anthropicMetadataSafe(raw json.RawMessage) bool {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || bytes.Equal(bytes.TrimSpace(raw), []byte("{}")) {
		return true
	}
	var value struct {
		UserID string `json:"user_id,omitempty"`
	}
	return decodeStrict(raw, &value) == nil && len(value.UserID) <= 256
}

func anthropicMessageToChat(message anthropicMessage) ([]Message, error) {
	if message.Role != "user" && message.Role != "assistant" {
		return nil, invalidProtocolRequest("Anthropic Messages", "message role is unsupported")
	}
	var text string
	if json.Unmarshal(message.Content, &text) == nil {
		if text == "" {
			return nil, invalidProtocolRequest("Anthropic Messages", "message text must not be empty")
		}
		return []Message{{Role: message.Role, Content: rawString(text)}}, nil
	}
	var blocks []anthropicContentBlock
	if decodeStrict(message.Content, &blocks) != nil || len(blocks) == 0 || len(blocks) > maxMessageParts {
		return nil, invalidProtocolRequest("Anthropic Messages", "message content blocks are invalid")
	}
	if message.Role == "assistant" {
		texts := make([]string, 0)
		calls := make([]ToolCall, 0)
		for _, block := range blocks {
			if hasNonNull(block.CacheControl) {
				return nil, unsupportedProtocolField("Anthropic Messages", "messages.content.cache_control")
			}
			switch block.Type {
			case "text":
				if block.Text == "" {
					return nil, invalidProtocolRequest("Anthropic Messages", "text block must not be empty")
				}
				texts = append(texts, block.Text)
			case "tool_use":
				if !validToolCallID(block.ID) || !validToolName(block.Name) || !rawJSONObject(block.Input) {
					return nil, invalidProtocolRequest("Anthropic Messages", "tool_use block is invalid")
				}
				arguments := string(bytes.TrimSpace(block.Input))
				calls = append(calls, ToolCall{ID: block.ID, Type: "function", Function: ToolFunction{Name: block.Name, Arguments: arguments}})
			default:
				return nil, unsupportedProtocolField("Anthropic Messages", "messages.content."+block.Type)
			}
		}
		content := rawNull()
		if len(texts) != 0 {
			content = joinTextParts(texts)
		}
		return []Message{{Role: "assistant", Content: content, ToolCalls: calls}}, nil
	}

	result := make([]Message, 0, len(blocks))
	texts := make([]string, 0)
	for _, block := range blocks {
		if hasNonNull(block.CacheControl) {
			return nil, unsupportedProtocolField("Anthropic Messages", "messages.content.cache_control")
		}
		switch block.Type {
		case "text":
			if block.Text == "" {
				return nil, invalidProtocolRequest("Anthropic Messages", "text block must not be empty")
			}
			texts = append(texts, block.Text)
		case "tool_result":
			if !validToolCallID(block.ToolUseID) {
				return nil, invalidProtocolRequest("Anthropic Messages", "tool_result tool_use_id is invalid")
			}
			content, err := anthropicToolResultText(block.Content)
			if err != nil || content == "" {
				return nil, invalidProtocolRequest("Anthropic Messages", "tool_result must contain text")
			}
			result = append(result, Message{Role: "tool", Content: rawString(content), ToolCallID: block.ToolUseID})
		default:
			return nil, unsupportedProtocolField("Anthropic Messages", "messages.content."+block.Type)
		}
	}
	if len(texts) != 0 {
		result = append(result, Message{Role: "user", Content: joinTextParts(texts)})
	}
	if len(result) == 0 {
		return nil, invalidProtocolRequest("Anthropic Messages", "user message contains no supported content")
	}
	return result, nil
}

func anthropicTextBlocks(raw json.RawMessage) ([]string, error) {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		if text == "" {
			return nil, errors.New("empty text")
		}
		return []string{text}, nil
	}
	var blocks []anthropicContentBlock
	if decodeStrict(raw, &blocks) != nil || len(blocks) == 0 || len(blocks) > maxMessageParts {
		return nil, errors.New("invalid text blocks")
	}
	result := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if hasNonNull(block.CacheControl) {
			return nil, unsupportedProtocolField("Anthropic Messages", "system.cache_control")
		}
		if block.Type != "text" || block.Text == "" {
			return nil, errors.New("unsupported text block")
		}
		result = append(result, block.Text)
	}
	return result, nil
}

func anthropicToolResultText(raw json.RawMessage) (string, error) {
	return responsesTextContent(raw, true)
}

func anthropicToolChoice(raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	var value struct {
		Type                   string `json:"type"`
		Name                   string `json:"name,omitempty"`
		DisableParallelToolUse *bool  `json:"disable_parallel_tool_use,omitempty"`
	}
	if decodeStrict(raw, &value) != nil {
		return nil, invalidProtocolRequest("Anthropic Messages", "tool_choice is invalid")
	}
	switch value.Type {
	case "auto":
		return rawString("auto"), nil
	case "any":
		return rawString("required"), nil
	case "none":
		return rawString("none"), nil
	case "tool":
		if !validToolName(value.Name) {
			return nil, invalidProtocolRequest("Anthropic Messages", "tool_choice name is invalid")
		}
		choice := namedToolChoice{Type: "function"}
		choice.Function.Name = value.Name
		data, _ := json.Marshal(choice)
		return data, nil
	default:
		return nil, invalidProtocolRequest("Anthropic Messages", "tool_choice is unsupported")
	}
}

func hasNonNull(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) != 0 && !bytes.Equal(trimmed, []byte("null"))
}

func rawJSONObject(raw json.RawMessage) bool {
	var value map[string]interface{}
	return decodeJSONValue(raw, &value) == nil && value != nil
}

func encodeAnthropicResponse(body []byte, requestID, publicModel string, now time.Time) ([]byte, error) {
	chat, err := parseChatCompletionForConversion(body)
	if err != nil {
		return nil, err
	}
	id := safeProviderID(chat.ID)
	if id == "" {
		id = requestID
	}
	choice := chat.Choices[0]
	content := make([]interface{}, 0, 1+len(choice.Message.ToolCalls))
	if choice.Message.Content != nil {
		content = append(content, map[string]interface{}{"type": "text", "text": *choice.Message.Content})
	}
	for _, call := range choice.Message.ToolCalls {
		var input map[string]interface{}
		if json.Unmarshal([]byte(call.Function.Arguments), &input) != nil {
			return nil, errors.New("tool arguments are not an object")
		}
		content = append(content, map[string]interface{}{"type": "tool_use", "id": call.ID, "name": call.Function.Name, "input": input})
	}
	stopReason := "end_turn"
	switch choice.FinishReason {
	case "length":
		stopReason = "max_tokens"
	case "tool_calls":
		stopReason = "tool_use"
	case "stop", "":
	default:
		return nil, fmt.Errorf("unsupported finish reason %q", choice.FinishReason)
	}
	inputTokens, outputTokens, cachedTokens := int64(0), int64(0), int64(0)
	if chat.Usage != nil {
		if chat.Usage.PromptTokens != nil {
			inputTokens = *chat.Usage.PromptTokens
		}
		if chat.Usage.CompletionTokens != nil {
			outputTokens = *chat.Usage.CompletionTokens
		}
		if chat.Usage.CachedTokens != nil {
			cachedTokens = *chat.Usage.CachedTokens
		}
		if chat.Usage.PromptTokensDetails != nil && chat.Usage.PromptTokensDetails.CachedTokens != nil {
			cachedTokens = *chat.Usage.PromptTokensDetails.CachedTokens
		}
	}
	response := map[string]interface{}{
		"id": id, "type": "message", "role": "assistant", "model": publicModel,
		"content": content, "stop_reason": stopReason, "stop_sequence": nil,
		"usage": map[string]interface{}{"input_tokens": inputTokens, "output_tokens": outputTokens, "cache_read_input_tokens": cachedTokens, "cache_creation_input_tokens": 0},
	}
	_ = now // retained for parity with response encoders and deterministic tests.
	return json.Marshal(response)
}

func writeAnthropicEvent(eventType string, payload interface{}) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return []byte("event: " + eventType + "\ndata: " + string(data) + "\n\n"), nil
}
