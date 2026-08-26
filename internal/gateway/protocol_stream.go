package gateway

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type protocolStreamEncoder interface {
	Encode(string) ([]byte, error)
	Finish(streamState) ([]byte, error)
}

func newProtocolStreamEncoder(protocol wireProtocol, requestID, publicModel string, now time.Time, aliases map[string]responsesToolIdentity) protocolStreamEncoder {
	switch protocol {
	case protocolResponses:
		return &responsesStreamEncoder{requestID: requestID, publicModel: publicModel, createdAt: now.Unix(), tools: make(map[int]*translatedTool), aliases: aliases}
	case protocolAnthropic:
		return &anthropicStreamEncoder{requestID: requestID, publicModel: publicModel, tools: make(map[int]*translatedTool), openBlock: -1}
	default:
		return nil
	}
}

type translatedTool struct {
	index       int
	outputIndex int
	itemID      string
	callID      string
	name        string
	namespace   string
	arguments   strings.Builder
	blockIndex  int
}

type streamDelta struct {
	Role      string                  `json:"role"`
	Content   *string                 `json:"content"`
	ToolCalls []providerToolCallDelta `json:"tool_calls"`
}

func decodeTranslatedStreamChunk(data string) (providerStreamChunk, providerStreamChoice, streamDelta, error) {
	var chunk providerStreamChunk
	if err := decodeJSONValue([]byte(data), &chunk); err != nil {
		return chunk, providerStreamChoice{}, streamDelta{}, err
	}
	if len(chunk.Choices) == 0 || bytes.Equal(bytes.TrimSpace(chunk.Choices), []byte("[]")) {
		if chunk.Usage != nil {
			return chunk, providerStreamChoice{}, streamDelta{}, nil
		}
		return chunk, providerStreamChoice{}, streamDelta{}, errors.New("translated stream chunk has no choice")
	}
	var choices []providerStreamChoice
	if json.Unmarshal(chunk.Choices, &choices) != nil || len(choices) != 1 || choices[0].Index != 0 {
		return chunk, providerStreamChoice{}, streamDelta{}, errors.New("translated protocols require one stream choice")
	}
	var delta streamDelta
	if len(choices[0].Delta) != 0 && !bytes.Equal(bytes.TrimSpace(choices[0].Delta), []byte("null")) {
		if json.Unmarshal(choices[0].Delta, &delta) != nil {
			return chunk, providerStreamChoice{}, streamDelta{}, errors.New("invalid translated stream delta")
		}
	}
	return chunk, choices[0], delta, nil
}

type responsesStreamEncoder struct {
	requestID, publicModel string
	createdAt              int64
	responseID             string
	sequence               int
	started                bool
	textStarted            bool
	textOutputIndex        int
	textItemID             string
	text                   strings.Builder
	tools                  map[int]*translatedTool
	aliases                map[string]responsesToolIdentity
	nextOutputIndex        int
	finishReason           string
}

func (e *responsesStreamEncoder) event(kind string, payload map[string]interface{}) ([]byte, error) {
	e.sequence++
	payload["type"] = kind
	payload["sequence_number"] = e.sequence
	return writeResponsesEvent(kind, payload)
}

func (e *responsesStreamEncoder) start(chunk providerStreamChunk) ([]byte, error) {
	if e.started {
		return nil, nil
	}
	e.started = true
	e.responseID = safeProviderID(chunk.ID)
	if e.responseID == "" {
		e.responseID = e.requestID
	}
	base := e.response("in_progress", nil, nil)
	created, err := e.event("response.created", map[string]interface{}{"response": base})
	if err != nil {
		return nil, err
	}
	inProgress, err := e.event("response.in_progress", map[string]interface{}{"response": base})
	return append(created, inProgress...), err
}

func (e *responsesStreamEncoder) Encode(data string) ([]byte, error) {
	chunk, choice, delta, err := decodeTranslatedStreamChunk(data)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	start, err := e.start(chunk)
	if err != nil {
		return nil, err
	}
	output.Write(start)
	if choice.FinishReason != nil && len(choice.FinishReason) != 0 && !bytes.Equal(bytes.TrimSpace(choice.FinishReason), []byte("null")) {
		if json.Unmarshal(choice.FinishReason, &e.finishReason) != nil {
			return nil, errors.New("invalid translated finish reason")
		}
	}
	if delta.Content != nil && *delta.Content != "" {
		if !e.textStarted {
			e.textStarted = true
			e.textOutputIndex = e.nextOutputIndex
			e.nextOutputIndex++
			e.textItemID = "msg_" + e.requestID
			item := map[string]interface{}{"type": "message", "id": e.textItemID, "status": "in_progress", "role": "assistant", "content": []interface{}{}}
			frame, frameErr := e.event("response.output_item.added", map[string]interface{}{"output_index": e.textOutputIndex, "item": item})
			if frameErr != nil {
				return nil, frameErr
			}
			output.Write(frame)
			frame, frameErr = e.event("response.content_part.added", map[string]interface{}{"item_id": e.textItemID, "output_index": e.textOutputIndex, "content_index": 0, "part": map[string]interface{}{"type": "output_text", "text": "", "annotations": []interface{}{}}})
			if frameErr != nil {
				return nil, frameErr
			}
			output.Write(frame)
		}
		e.text.WriteString(*delta.Content)
		frame, frameErr := e.event("response.output_text.delta", map[string]interface{}{"item_id": e.textItemID, "output_index": e.textOutputIndex, "content_index": 0, "delta": *delta.Content})
		if frameErr != nil {
			return nil, frameErr
		}
		output.Write(frame)
	}
	for _, call := range delta.ToolCalls {
		tool := e.tools[call.Index]
		if tool == nil {
			if !validToolCallID(call.ID) || call.Function == nil || !validToolName(call.Function.Name) {
				return nil, errors.New("translated tool stream did not begin with an id and name")
			}
			tool = &translatedTool{index: call.Index, outputIndex: e.nextOutputIndex, itemID: fmt.Sprintf("fc_%s_%d", e.requestID, call.Index), callID: call.ID, name: call.Function.Name}
			public := responsesFunctionCall(tool.name, e.aliases)
			tool.name = public["name"].(string)
			if namespace, ok := public["namespace"].(string); ok {
				tool.namespace = namespace
			}
			e.nextOutputIndex++
			e.tools[call.Index] = tool
			item := map[string]interface{}{"type": "function_call", "id": tool.itemID, "call_id": tool.callID, "name": tool.name, "arguments": "", "status": "in_progress"}
			if tool.namespace != "" {
				item["namespace"] = tool.namespace
			}
			frame, frameErr := e.event("response.output_item.added", map[string]interface{}{"output_index": tool.outputIndex, "item": item})
			if frameErr != nil {
				return nil, frameErr
			}
			output.Write(frame)
		} else {
			if call.ID != "" && call.ID != tool.callID {
				return nil, errors.New("translated tool id changed during stream")
			}
			if call.Function != nil && call.Function.Name != "" {
				public := responsesFunctionCall(call.Function.Name, e.aliases)
				if public["name"] != tool.name {
					return nil, errors.New("translated tool name changed during stream")
				}
			}
		}
		if call.Function != nil && call.Function.Arguments != "" {
			tool.arguments.WriteString(call.Function.Arguments)
			frame, frameErr := e.event("response.function_call_arguments.delta", map[string]interface{}{"item_id": tool.itemID, "output_index": tool.outputIndex, "delta": call.Function.Arguments})
			if frameErr != nil {
				return nil, frameErr
			}
			output.Write(frame)
		}
	}
	return output.Bytes(), nil
}

func (e *responsesStreamEncoder) Finish(meta streamState) ([]byte, error) {
	if !e.started {
		return nil, errors.New("translated response stream never started")
	}
	var output bytes.Buffer
	items := make([]indexedItem, 0, e.nextOutputIndex)
	if e.textStarted {
		text := e.text.String()
		frame, err := e.event("response.output_text.done", map[string]interface{}{"item_id": e.textItemID, "output_index": e.textOutputIndex, "content_index": 0, "text": text})
		if err != nil {
			return nil, err
		}
		output.Write(frame)
		part := map[string]interface{}{"type": "output_text", "text": text, "annotations": []interface{}{}}
		frame, err = e.event("response.content_part.done", map[string]interface{}{"item_id": e.textItemID, "output_index": e.textOutputIndex, "content_index": 0, "part": part})
		if err != nil {
			return nil, err
		}
		output.Write(frame)
		item := map[string]interface{}{"type": "message", "id": e.textItemID, "status": "completed", "role": "assistant", "content": []interface{}{part}}
		frame, err = e.event("response.output_item.done", map[string]interface{}{"output_index": e.textOutputIndex, "item": item})
		if err != nil {
			return nil, err
		}
		output.Write(frame)
		items = append(items, indexedItem{e.textOutputIndex, item})
	}
	indices := make([]int, 0, len(e.tools))
	for index := range e.tools {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	for _, index := range indices {
		tool := e.tools[index]
		arguments := tool.arguments.String()
		if !validJSONObjectString(arguments) {
			return nil, errors.New("translated tool arguments did not form a JSON object")
		}
		frame, err := e.event("response.function_call_arguments.done", map[string]interface{}{"item_id": tool.itemID, "output_index": tool.outputIndex, "arguments": arguments})
		if err != nil {
			return nil, err
		}
		output.Write(frame)
		item := map[string]interface{}{"type": "function_call", "id": tool.itemID, "call_id": tool.callID, "name": tool.name, "arguments": arguments, "status": "completed"}
		if tool.namespace != "" {
			item["namespace"] = tool.namespace
		}
		frame, err = e.event("response.output_item.done", map[string]interface{}{"output_index": tool.outputIndex, "item": item})
		if err != nil {
			return nil, err
		}
		output.Write(frame)
		items = append(items, indexedItem{tool.outputIndex, item})
	}
	if len(items) == 0 {
		return nil, errors.New("translated response stream contained no output")
	}
	sort.Slice(items, func(i, j int) bool { return items[i].index < items[j].index })
	ordered := make([]interface{}, 0, len(items))
	for _, item := range items {
		ordered = append(ordered, item.value)
	}
	status := "completed"
	var incomplete interface{}
	if e.finishReason == "length" {
		status = "incomplete"
		incomplete = map[string]interface{}{"reason": "max_output_tokens"}
	} else if e.finishReason != "stop" && e.finishReason != "tool_calls" {
		return nil, fmt.Errorf("unsupported translated finish reason %q", e.finishReason)
	}
	response := e.response(status, ordered, usageFromPlatform(meta.usage))
	response["incomplete_details"] = incomplete
	frame, err := e.event("response."+status, map[string]interface{}{"response": response})
	if err != nil {
		return nil, err
	}
	output.Write(frame)
	return output.Bytes(), nil
}

type indexedItem struct {
	index int
	value interface{}
}

func (e *responsesStreamEncoder) response(status string, output []interface{}, usage interface{}) map[string]interface{} {
	if output == nil {
		output = []interface{}{}
	}
	return map[string]interface{}{
		"id": e.responseID, "object": "response", "created_at": e.createdAt, "status": status,
		"error": nil, "incomplete_details": nil, "instructions": nil, "model": e.publicModel,
		"output": output, "parallel_tool_calls": true, "tool_choice": "auto", "tools": []interface{}{}, "usage": usage,
	}
}

type anthropicStreamEncoder struct {
	requestID, publicModel string
	messageID              string
	started                bool
	textStarted            bool
	textBlock              int
	text                   strings.Builder
	tools                  map[int]*translatedTool
	nextBlock              int
	openBlock              int
	finishReason           string
}

func (e *anthropicStreamEncoder) start(chunk providerStreamChunk) ([]byte, error) {
	if e.started {
		return nil, nil
	}
	e.started = true
	e.messageID = safeProviderID(chunk.ID)
	if e.messageID == "" {
		e.messageID = e.requestID
	}
	return writeAnthropicEvent("message_start", map[string]interface{}{
		"type": "message_start", "message": map[string]interface{}{
			"id": e.messageID, "type": "message", "role": "assistant", "model": e.publicModel,
			"content": []interface{}{}, "stop_reason": nil, "stop_sequence": nil,
			"usage": map[string]interface{}{"input_tokens": 0, "output_tokens": 0, "cache_read_input_tokens": 0, "cache_creation_input_tokens": 0},
		},
	})
}

func (e *anthropicStreamEncoder) closeOpen() ([]byte, error) {
	if e.openBlock < 0 {
		return nil, nil
	}
	index := e.openBlock
	e.openBlock = -1
	return writeAnthropicEvent("content_block_stop", map[string]interface{}{"type": "content_block_stop", "index": index})
}

func (e *anthropicStreamEncoder) Encode(data string) ([]byte, error) {
	chunk, choice, delta, err := decodeTranslatedStreamChunk(data)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	start, err := e.start(chunk)
	if err != nil {
		return nil, err
	}
	output.Write(start)
	if choice.FinishReason != nil && len(choice.FinishReason) != 0 && !bytes.Equal(bytes.TrimSpace(choice.FinishReason), []byte("null")) {
		if json.Unmarshal(choice.FinishReason, &e.finishReason) != nil {
			return nil, errors.New("invalid translated finish reason")
		}
	}
	if delta.Content != nil && *delta.Content != "" {
		if !e.textStarted {
			closed, closeErr := e.closeOpen()
			if closeErr != nil {
				return nil, closeErr
			}
			output.Write(closed)
			e.textStarted = true
			e.textBlock = e.nextBlock
			e.nextBlock++
			e.openBlock = e.textBlock
			frame, frameErr := writeAnthropicEvent("content_block_start", map[string]interface{}{"type": "content_block_start", "index": e.textBlock, "content_block": map[string]interface{}{"type": "text", "text": ""}})
			if frameErr != nil {
				return nil, frameErr
			}
			output.Write(frame)
		}
		e.text.WriteString(*delta.Content)
		frame, frameErr := writeAnthropicEvent("content_block_delta", map[string]interface{}{"type": "content_block_delta", "index": e.textBlock, "delta": map[string]interface{}{"type": "text_delta", "text": *delta.Content}})
		if frameErr != nil {
			return nil, frameErr
		}
		output.Write(frame)
	}
	for _, call := range delta.ToolCalls {
		tool := e.tools[call.Index]
		if tool == nil {
			if !validToolCallID(call.ID) || call.Function == nil || !validToolName(call.Function.Name) {
				return nil, errors.New("translated tool stream did not begin with an id and name")
			}
			closed, closeErr := e.closeOpen()
			if closeErr != nil {
				return nil, closeErr
			}
			output.Write(closed)
			tool = &translatedTool{index: call.Index, callID: call.ID, name: call.Function.Name, blockIndex: e.nextBlock}
			e.nextBlock++
			e.openBlock = tool.blockIndex
			e.tools[call.Index] = tool
			frame, frameErr := writeAnthropicEvent("content_block_start", map[string]interface{}{"type": "content_block_start", "index": tool.blockIndex, "content_block": map[string]interface{}{"type": "tool_use", "id": tool.callID, "name": tool.name, "input": map[string]interface{}{}}})
			if frameErr != nil {
				return nil, frameErr
			}
			output.Write(frame)
		}
		if call.Function != nil && call.Function.Arguments != "" {
			tool.arguments.WriteString(call.Function.Arguments)
			frame, frameErr := writeAnthropicEvent("content_block_delta", map[string]interface{}{"type": "content_block_delta", "index": tool.blockIndex, "delta": map[string]interface{}{"type": "input_json_delta", "partial_json": call.Function.Arguments}})
			if frameErr != nil {
				return nil, frameErr
			}
			output.Write(frame)
		}
	}
	return output.Bytes(), nil
}

func (e *anthropicStreamEncoder) Finish(meta streamState) ([]byte, error) {
	if !e.started {
		return nil, errors.New("translated Anthropic stream never started")
	}
	for _, tool := range e.tools {
		if !validJSONObjectString(tool.arguments.String()) {
			return nil, errors.New("translated tool arguments did not form a JSON object")
		}
	}
	var output bytes.Buffer
	closed, err := e.closeOpen()
	if err != nil {
		return nil, err
	}
	output.Write(closed)
	stopReason := "end_turn"
	switch e.finishReason {
	case "stop":
	case "length":
		stopReason = "max_tokens"
	case "tool_calls":
		stopReason = "tool_use"
	default:
		return nil, fmt.Errorf("unsupported translated finish reason %q", e.finishReason)
	}
	outputTokens := int64(0)
	if meta.usage.OutputTokens != nil {
		outputTokens = *meta.usage.OutputTokens
	}
	frame, err := writeAnthropicEvent("message_delta", map[string]interface{}{"type": "message_delta", "delta": map[string]interface{}{"stop_reason": stopReason, "stop_sequence": nil}, "usage": map[string]interface{}{"output_tokens": outputTokens}})
	if err != nil {
		return nil, err
	}
	output.Write(frame)
	frame, err = writeAnthropicEvent("message_stop", map[string]interface{}{"type": "message_stop"})
	if err != nil {
		return nil, err
	}
	output.Write(frame)
	return output.Bytes(), nil
}
