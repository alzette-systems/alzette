package gateway

// This file implements the OpenAI Responses <-> Chat Completions adapter used
// by Alzette. Bifrost owns the evolving Responses wire schema and the protocol
// mux. Alzette applies its policy after that normalization and remains
// authoritative for credentials, model routing, retries, and accounting.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"alzette/internal/platform"

	bifrostopenai "github.com/maximhq/bifrost/core/providers/openai"
	"github.com/maximhq/bifrost/core/schemas"
)

type responsesToolIdentity struct {
	Namespace string
	Name      string
}

func decodeResponsesRequest(data []byte) (ChatRequest, error) {
	var envelope map[string]json.RawMessage
	if err := decodeOneJSON(data, &envelope); err != nil || envelope == nil {
		return ChatRequest{}, invalidProtocolRequest("Responses", "is not valid JSON")
	}
	if raw, ok := envelope["input"]; !ok || len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return ChatRequest{}, invalidProtocolRequest("Responses", "requires input")
	}

	var wire bifrostopenai.OpenAIResponsesRequest
	if err := decodeOneJSON(data, &wire); err != nil {
		return ChatRequest{}, invalidProtocolRequest("Responses", "is not a valid Responses request")
	}
	if err := validateResponsesPolicy(envelope, &wire); err != nil {
		return ChatRequest{}, err
	}

	ctx := schemas.NewBifrostContext(context.Background(), time.Time{})
	request := wire.ToBifrostResponsesRequest(ctx)
	if request == nil || request.Params == nil {
		return ChatRequest{}, invalidProtocolRequest("Responses", "could not be normalized")
	}

	additionalTools, filteredInput, err := hoistResponsesAdditionalTools(request.Input)
	if err != nil {
		return ChatRequest{}, err
	}
	request.Input = filteredInput
	allTools := append(append(make([]schemas.ResponsesTool, 0, len(request.Params.Tools)+len(additionalTools)), request.Params.Tools...), additionalTools...)
	tools, aliases, identities, err := responsesToolsToChat(allTools)
	if err != nil {
		return ChatRequest{}, err
	}
	if err := rewriteResponsesNamespaceHistory(request.Input, identities); err != nil {
		return ChatRequest{}, err
	}

	chat := request.ToChatRequest()
	chat.Provider = schemas.DeepSeek
	chat.Model = wire.Model
	chatWire := bifrostopenai.ToOpenAIChatRequest(ctx, chat)
	if chatWire == nil {
		return ChatRequest{}, invalidProtocolRequest("Responses", "could not be mapped to the configured model API")
	}
	chatWire.Stream = wire.Stream
	encoded, err := json.Marshal(chatWire)
	if err != nil {
		return ChatRequest{}, invalidProtocolRequest("Responses", "could not be normalized")
	}
	var normalized ChatRequest
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		return ChatRequest{}, invalidProtocolRequest("Responses", "could not be normalized")
	}
	// Namespace tools are a Responses-only construct. Bifrost deliberately
	// drops them during the generic Chat mux, so Alzette supplies the flattened
	// functions parsed from Bifrost's typed tool schema and remembers how to
	// restore their public identities in the response.
	normalized.Model = wire.Model
	normalized.Stream = wire.Stream
	normalized.Tools = tools
	normalized.ResponsesToolAliases = aliases
	if normalized.streaming() {
		include := true
		normalized.StreamOptions = &StreamOptions{IncludeUsage: &include}
	}
	return normalized, nil
}

func validateResponsesPolicy(envelope map[string]json.RawMessage, input *bifrostopenai.OpenAIResponsesRequest) error {
	if input.PreviousResponseID != nil && *input.PreviousResponseID != "" {
		return unsupportedProtocolField("Responses", "previous_response_id")
	}
	if input.Store != nil && *input.Store {
		return unsupportedProtocolField("Responses", "store=true")
	}
	if len(input.Include) != 0 {
		return unsupportedProtocolField("Responses", "include")
	}
	if input.Background != nil && *input.Background {
		return unsupportedProtocolField("Responses", "background=true")
	}
	if input.Conversation != nil && *input.Conversation != "" {
		return unsupportedProtocolField("Responses", "conversation")
	}
	if raw, ok := envelope["prompt"]; ok && len(bytes.TrimSpace(raw)) != 0 && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return unsupportedProtocolField("Responses", "prompt")
	}
	if input.Text != nil && input.Text.Format != nil && input.Text.Format.Type != "" && input.Text.Format.Type != "text" {
		return unsupportedProtocolField("Responses", "text.format")
	}
	if input.Truncation != nil && *input.Truncation != "" && *input.Truncation != "disabled" {
		return unsupportedProtocolField("Responses", "truncation")
	}
	if raw := bytes.TrimSpace(input.ContextManagement); len(raw) != 0 && !bytes.Equal(raw, []byte("null")) && !bytes.Equal(raw, []byte("{}")) {
		return unsupportedProtocolField("Responses", "context_management")
	}
	return nil
}

func hoistResponsesAdditionalTools(input []schemas.ResponsesMessage) ([]schemas.ResponsesTool, []schemas.ResponsesMessage, error) {
	tools := make([]schemas.ResponsesTool, 0)
	filtered := make([]schemas.ResponsesMessage, 0, len(input))
	for _, message := range input {
		if message.Type == nil || *message.Type != schemas.ResponsesMessageTypeAdditionalTools {
			filtered = append(filtered, message)
			continue
		}
		if len(message.AdditionalTools) == 0 {
			continue
		}
		var declared []schemas.ResponsesTool
		if err := json.Unmarshal(message.AdditionalTools, &declared); err != nil {
			return nil, nil, invalidProtocolRequest("Responses", "additional_tools contains invalid tools")
		}
		tools = append(tools, declared...)
	}
	return tools, filtered, nil
}

func rewriteResponsesNamespaceHistory(input []schemas.ResponsesMessage, identities map[string]string) error {
	for index := range input {
		message := &input[index]
		if message.Type == nil || *message.Type != schemas.ResponsesMessageTypeFunctionCall || message.ResponsesToolMessage == nil || message.Namespace == nil || *message.Namespace == "" {
			continue
		}
		if message.Name == nil || *message.Name == "" {
			return invalidProtocolRequest("Responses", "namespace function_call is missing a name")
		}
		alias, ok := identities[responsesIdentityKey(*message.Namespace, *message.Name)]
		if !ok {
			return invalidProtocolRequest("Responses", "function_call references an undeclared namespace tool")
		}
		message.Name = schemas.Ptr(alias)
		message.Namespace = nil
	}
	return nil
}

func responsesToolsToChat(values []schemas.ResponsesTool) ([]Tool, map[string]responsesToolIdentity, map[string]string, error) {
	tools := make([]Tool, 0, len(values))
	aliases := make(map[string]responsesToolIdentity)
	identities := make(map[string]string)
	used := make(map[string]struct{})

	addFunction := func(value schemas.ResponsesTool, namespace string) error {
		if value.Name == nil || !validToolName(*value.Name) || value.ResponsesToolFunction == nil {
			return nil
		}
		name := *value.Name
		alias := name
		if namespace != "" {
			if !validToolName(namespace) {
				return nil
			}
			alias = namespace + name
			if !strings.HasSuffix(namespace, "_") {
				alias = namespace + "__" + name
			}
			if !validToolName(alias) {
				sum := sha256.Sum256([]byte(responsesIdentityKey(namespace, name)))
				alias = fmt.Sprintf("alz_ns_%x", sum[:12])
			}
		}
		if _, exists := used[alias]; exists {
			if namespace == "" {
				return invalidProtocolRequest("Responses", "declares duplicate function tools")
			}
			sum := sha256.Sum256([]byte(responsesIdentityKey(namespace, name)))
			alias = fmt.Sprintf("alz_ns_%x", sum[:12])
			if _, collision := used[alias]; collision {
				return invalidProtocolRequest("Responses", "declares colliding namespace tools")
			}
		}
		parameters := json.RawMessage(`{"type":"object","properties":{}}`)
		if value.ResponsesToolFunction.Parameters != nil {
			encoded, marshalErr := json.Marshal(value.ResponsesToolFunction.Parameters)
			if marshalErr != nil {
				return invalidProtocolRequest("Responses", "contains invalid function parameters")
			}
			parameters = encoded
		}
		description := ""
		if value.Description != nil {
			description = *value.Description
		}
		tools = append(tools, Tool{Type: "function", Function: ToolDefinition{Name: alias, Description: description, Parameters: parameters, Strict: value.ResponsesToolFunction.Strict}})
		used[alias] = struct{}{}
		if namespace != "" {
			identity := responsesToolIdentity{Namespace: namespace, Name: name}
			aliases[alias] = identity
			identities[responsesIdentityKey(namespace, name)] = alias
		}
		return nil
	}

	for _, value := range values {
		switch value.Type {
		case schemas.ResponsesToolTypeFunction:
			if err := addFunction(value, ""); err != nil {
				return nil, nil, nil, err
			}
		case schemas.ResponsesToolTypeNamespace:
			if value.Name == nil || value.ResponsesToolNamespace == nil {
				continue
			}
			for _, child := range value.ResponsesToolNamespace.Tools {
				if child.Type == schemas.ResponsesToolTypeFunction {
					if err := addFunction(child, *value.Name); err != nil {
						return nil, nil, nil, err
					}
				}
			}
		}
		if len(tools) > maxTools {
			return nil, nil, nil, invalidProtocolRequest("Responses", "declares too many compatible function tools")
		}
	}
	return tools, aliases, identities, nil
}

func responsesIdentityKey(namespace, name string) string { return namespace + "\x00" + name }

func responsesFunctionCall(name string, aliases map[string]responsesToolIdentity) map[string]interface{} {
	result := map[string]interface{}{"name": name}
	if identity, ok := aliases[name]; ok {
		result["name"] = identity.Name
		result["namespace"] = identity.Namespace
	}
	return result
}

// responsesTextContent is shared by the Anthropic tool-result adapter. Bifrost
// owns Responses input decoding; this helper only normalizes the small textual
// result union accepted by the internal Chat representation.
func responsesTextContent(raw json.RawMessage, allowPlain bool) (string, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if !allowPlain && text == "" {
			return "", errors.New("empty text")
		}
		return text, nil
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	}
	if err := decodeStrict(raw, &parts); err != nil || len(parts) == 0 || len(parts) > maxMessageParts {
		return "", errors.New("unsupported text content")
	}
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		if (part.Type != "input_text" && part.Type != "output_text" && part.Type != "text") || part.Text == "" {
			return "", errors.New("unsupported text part")
		}
		texts = append(texts, part.Text)
	}
	return strings.Join(texts, "\n"), nil
}

type chatCompletionResponse struct {
	ID      string                 `json:"id"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []chatCompletionChoice `json:"choices"`
	Usage   *providerUsage         `json:"usage"`
}

type chatCompletionChoice struct {
	Index        int                   `json:"index"`
	Message      chatCompletionMessage `json:"message"`
	FinishReason string                `json:"finish_reason"`
}

type chatCompletionMessage struct {
	Role      string     `json:"role"`
	Content   *string    `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

func parseChatCompletionForConversion(body []byte) (chatCompletionResponse, error) {
	var response chatCompletionResponse
	if err := decodeOneJSON(body, &response); err != nil || len(response.Choices) != 1 {
		return chatCompletionResponse{}, errors.New("translated protocols require exactly one completion choice")
	}
	choice := response.Choices[0]
	if choice.Index != 0 || choice.Message.Role != "assistant" {
		return chatCompletionResponse{}, errors.New("translated response contains an invalid assistant choice")
	}
	if choice.Message.Content == nil && len(choice.Message.ToolCalls) == 0 {
		return chatCompletionResponse{}, errors.New("translated response contains no output")
	}
	for _, call := range choice.Message.ToolCalls {
		if call.Type != "function" || !validToolCallID(call.ID) || !validToolName(call.Function.Name) || !validJSONObjectString(call.Function.Arguments) {
			return chatCompletionResponse{}, errors.New("translated response contains an invalid function call")
		}
	}
	return response, nil
}

func encodeResponsesResponse(body []byte, requestID, publicModel string, now time.Time, aliases map[string]responsesToolIdentity) ([]byte, error) {
	chat, err := parseChatCompletionForConversion(body)
	if err != nil {
		return nil, err
	}
	id := safeProviderID(chat.ID)
	if id == "" {
		id = requestID
	}
	created := chat.Created
	if created <= 0 {
		created = now.Unix()
	}
	choice := chat.Choices[0]
	output := make([]interface{}, 0, 1+len(choice.Message.ToolCalls))
	if choice.Message.Content != nil {
		output = append(output, map[string]interface{}{
			"type": "message", "id": "msg_" + requestID, "status": "completed", "role": "assistant",
			"content": []interface{}{map[string]interface{}{"type": "output_text", "text": *choice.Message.Content, "annotations": []interface{}{}}},
		})
	}
	for index, call := range choice.Message.ToolCalls {
		item := map[string]interface{}{
			"type": "function_call", "id": fmt.Sprintf("fc_%s_%d", requestID, index), "call_id": call.ID,
			"name": call.Function.Name, "arguments": call.Function.Arguments, "status": "completed",
		}
		for key, value := range responsesFunctionCall(call.Function.Name, aliases) {
			item[key] = value
		}
		output = append(output, item)
	}
	status := "completed"
	var incomplete interface{}
	if choice.FinishReason == "length" {
		status = "incomplete"
		incomplete = map[string]interface{}{"reason": "max_output_tokens"}
	}
	usage := responsesUsage(chat.Usage)
	response := map[string]interface{}{
		"id": id, "object": "response", "created_at": created, "status": status,
		"error": nil, "incomplete_details": incomplete, "instructions": nil,
		"model": publicModel, "output": output, "parallel_tool_calls": true,
		"tool_choice": "auto", "tools": []interface{}{}, "usage": usage,
	}
	return json.Marshal(response)
}

func responsesUsage(value *providerUsage) map[string]interface{} {
	input, output, cached, reasoning := int64(0), int64(0), int64(0), int64(0)
	if value != nil {
		if value.PromptTokens != nil {
			input = *value.PromptTokens
		}
		if value.CompletionTokens != nil {
			output = *value.CompletionTokens
		}
		if value.CachedTokens != nil {
			cached = *value.CachedTokens
		}
		if value.PromptTokensDetails != nil && value.PromptTokensDetails.CachedTokens != nil {
			cached = *value.PromptTokensDetails.CachedTokens
		}
		if value.ReasoningTokens != nil {
			reasoning = *value.ReasoningTokens
		}
		if value.CompletionTokensDetails != nil && value.CompletionTokensDetails.ReasoningTokens != nil {
			reasoning = *value.CompletionTokensDetails.ReasoningTokens
		}
	}
	return map[string]interface{}{
		"input_tokens": input, "output_tokens": output, "total_tokens": input + output,
		"input_tokens_details":  map[string]interface{}{"cached_tokens": cached},
		"output_tokens_details": map[string]interface{}{"reasoning_tokens": reasoning},
	}
}

func usageFromPlatform(value platform.TokenUsage) map[string]interface{} {
	usage := &providerUsage{PromptTokens: value.InputTokens, CompletionTokens: value.OutputTokens, CachedTokens: value.CachedTokens, ReasoningTokens: value.ReasoningTokens}
	return responsesUsage(usage)
}

func writeResponsesEvent(eventType string, payload interface{}) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return []byte("event: " + eventType + "\ndata: " + string(data) + "\n\n"), nil
}
