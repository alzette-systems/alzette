package gateway

// This file implements the bounded OpenAI Responses <-> Chat Completions
// conversion used by Alzette. It directly uses Maxim Bifrost's Apache-2.0
// Responses tool schemas and compatibility rules while keeping Alzette's
// request validation, routing, retry, and accounting invariants authoritative.

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"alzette/internal/platform"

	"github.com/maximhq/bifrost/core/schemas"
)

type responsesRequest struct {
	Model              string                  `json:"model"`
	Input              json.RawMessage         `json:"input"`
	Instructions       json.RawMessage         `json:"instructions,omitempty"`
	Stream             *bool                   `json:"stream,omitempty"`
	MaxOutputTokens    *int                    `json:"max_output_tokens,omitempty"`
	Temperature        *float64                `json:"temperature,omitempty"`
	TopP               *float64                `json:"top_p,omitempty"`
	Tools              []schemas.ResponsesTool `json:"tools,omitempty"`
	ToolChoice         json.RawMessage         `json:"tool_choice,omitempty"`
	ParallelToolCalls  *bool                   `json:"parallel_tool_calls,omitempty"`
	Reasoning          *responsesReasoning     `json:"reasoning,omitempty"`
	Store              *bool                   `json:"store,omitempty"`
	PreviousResponseID string                  `json:"previous_response_id,omitempty"`
	Include            []string                `json:"include,omitempty"`
	Metadata           json.RawMessage         `json:"metadata,omitempty"`
	Text               json.RawMessage         `json:"text,omitempty"`
	Truncation         string                  `json:"truncation,omitempty"`
	User               string                  `json:"user,omitempty"`
}

type responsesReasoning struct {
	Effort  string `json:"effort,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type responsesInputItem struct {
	Type      string                  `json:"type,omitempty"`
	Role      string                  `json:"role,omitempty"`
	Content   json.RawMessage         `json:"content,omitempty"`
	ID        string                  `json:"id,omitempty"`
	Status    string                  `json:"status,omitempty"`
	Phase     string                  `json:"phase,omitempty"`
	CallID    string                  `json:"call_id,omitempty"`
	Name      string                  `json:"name,omitempty"`
	Namespace string                  `json:"namespace,omitempty"`
	Arguments string                  `json:"arguments,omitempty"`
	Output    json.RawMessage         `json:"output,omitempty"`
	Tools     []schemas.ResponsesTool `json:"tools,omitempty"`
}

type responsesToolIdentity struct {
	Namespace string
	Name      string
}

type responsesContentPart struct {
	Type        string          `json:"type"`
	Text        string          `json:"text,omitempty"`
	Annotations json.RawMessage `json:"annotations,omitempty"`
}

func decodeResponsesRequest(data []byte) (ChatRequest, error) {
	var input responsesRequest
	if err := decodeStrict(data, &input); err != nil {
		if strings.Contains(err.Error(), "json: unknown field") {
			return ChatRequest{}, unsupportedProtocolField("Responses", strings.TrimPrefix(strings.TrimSpace(err.Error()), "json: unknown field "))
		}
		return ChatRequest{}, invalidProtocolRequest("Responses", "is not valid JSON")
	}
	if input.PreviousResponseID != "" {
		return ChatRequest{}, unsupportedProtocolField("Responses", "previous_response_id")
	}
	if input.Store != nil && *input.Store {
		return ChatRequest{}, unsupportedProtocolField("Responses", "store=true")
	}
	if len(input.Include) != 0 {
		return ChatRequest{}, unsupportedProtocolField("Responses", "include")
	}
	if len(bytes.TrimSpace(input.Metadata)) != 0 && !bytes.Equal(bytes.TrimSpace(input.Metadata), []byte("null")) && !bytes.Equal(bytes.TrimSpace(input.Metadata), []byte("{}")) {
		return ChatRequest{}, unsupportedProtocolField("Responses", "metadata")
	}
	if input.Text != nil && !responsesTextIsDefault(input.Text) {
		return ChatRequest{}, unsupportedProtocolField("Responses", "text.format")
	}
	if input.Truncation != "" && input.Truncation != "disabled" {
		return ChatRequest{}, unsupportedProtocolField("Responses", "truncation")
	}
	if input.User != "" {
		return ChatRequest{}, unsupportedProtocolField("Responses", "user")
	}

	messages, additionalTools, err := responsesInputToMessages(input.Instructions, input.Input)
	if err != nil {
		return ChatRequest{}, err
	}
	allTools := append(append(make([]schemas.ResponsesTool, 0, len(input.Tools)+len(additionalTools)), input.Tools...), additionalTools...)
	tools, aliases, identities, err := responsesToolsToChat(allTools)
	if err != nil {
		return ChatRequest{}, err
	}
	for messageIndex := range messages {
		for callIndex := range messages[messageIndex].ToolCalls {
			call := &messages[messageIndex].ToolCalls[callIndex]
			if call.Namespace == "" {
				continue
			}
			alias, ok := identities[responsesIdentityKey(call.Namespace, call.Function.Name)]
			if !ok {
				return ChatRequest{}, invalidProtocolRequest("Responses", "function_call references an undeclared namespace tool")
			}
			call.Function.Name = alias
		}
	}
	toolChoice, err := responsesToolChoice(input.ToolChoice)
	if err != nil {
		return ChatRequest{}, err
	}
	reasoningEffort := ""
	if input.Reasoning != nil {
		reasoningEffort = input.Reasoning.Effort
		if input.Reasoning.Summary != "" {
			return ChatRequest{}, unsupportedProtocolField("Responses", "reasoning.summary")
		}
	}
	request := ChatRequest{
		Model: input.Model, Messages: messages, Stream: input.Stream,
		Temperature: input.Temperature, TopP: input.TopP, MaxTokens: input.MaxOutputTokens,
		ReasoningEffort: reasoningEffort, ParallelToolCalls: input.ParallelToolCalls, Tools: tools, ToolChoice: toolChoice,
		ResponsesToolAliases: aliases,
	}
	if request.streaming() {
		include := true
		request.StreamOptions = &StreamOptions{IncludeUsage: &include}
	}
	return request, nil
}

func responsesTextIsDefault(raw json.RawMessage) bool {
	var value struct {
		Format json.RawMessage `json:"format,omitempty"`
	}
	if err := decodeStrict(raw, &value); err != nil {
		return false
	}
	if len(value.Format) == 0 || bytes.Equal(bytes.TrimSpace(value.Format), []byte("null")) {
		return true
	}
	var format struct {
		Type string `json:"type"`
	}
	return decodeStrict(value.Format, &format) == nil && (format.Type == "" || format.Type == "text")
}

func responsesInputToMessages(instructions, raw json.RawMessage) ([]Message, []schemas.ResponsesTool, error) {
	messages := make([]Message, 0)
	additionalTools := make([]schemas.ResponsesTool, 0)
	if len(bytes.TrimSpace(instructions)) != 0 && !bytes.Equal(bytes.TrimSpace(instructions), []byte("null")) {
		text, err := responsesTextContent(instructions, true)
		if err != nil || text == "" {
			return nil, nil, invalidProtocolRequest("Responses", "instructions must contain text")
		}
		messages = append(messages, Message{Role: "system", Content: rawString(text)})
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil, invalidProtocolRequest("Responses", "requires input")
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if text == "" {
			return nil, nil, invalidProtocolRequest("Responses", "input text must not be empty")
		}
		return append(messages, Message{Role: "user", Content: rawString(text)}), nil, nil
	}
	var items []responsesInputItem
	if err := decodeStrict(raw, &items); err != nil || len(items) == 0 || len(items) > maxMessages*2 {
		return nil, nil, invalidProtocolRequest("Responses", "input must be text or a supported item array")
	}
	for _, item := range items {
		switch item.Type {
		case "", "message":
			role := item.Role
			if role == "developer" {
				role = "system"
			}
			if role != "system" && role != "user" && role != "assistant" {
				return nil, nil, invalidProtocolRequest("Responses", "message role is unsupported")
			}
			content, err := responsesMessageContent(item.Content)
			if err != nil {
				return nil, nil, err
			}
			messages = append(messages, Message{Role: role, Content: content})
		case "function_call":
			if !validToolCallID(item.CallID) || !validToolName(item.Name) || !validJSONObjectString(item.Arguments) {
				return nil, nil, invalidProtocolRequest("Responses", "function_call is invalid")
			}
			call := ToolCall{ID: item.CallID, Type: "function", Function: ToolFunction{Name: item.Name, Arguments: item.Arguments}, Namespace: item.Namespace}
			if len(messages) != 0 && messages[len(messages)-1].Role == "assistant" && messages[len(messages)-1].ToolCallID == "" {
				last := &messages[len(messages)-1]
				last.ToolCalls = append(last.ToolCalls, call)
				if len(last.Content) == 0 {
					last.Content = rawNull()
				}
			} else {
				messages = append(messages, Message{Role: "assistant", Content: rawNull(), ToolCalls: []ToolCall{call}})
			}
		case "function_call_output":
			if !validToolCallID(item.CallID) {
				return nil, nil, invalidProtocolRequest("Responses", "function_call_output call_id is invalid")
			}
			output, err := responsesTextContent(item.Output, false)
			if err != nil || output == "" {
				return nil, nil, invalidProtocolRequest("Responses", "function_call_output must contain text")
			}
			messages = append(messages, Message{Role: "tool", Content: rawString(output), ToolCallID: item.CallID})
		case "additional_tools":
			if item.Role != "" && item.Role != "developer" {
				return nil, nil, invalidProtocolRequest("Responses", "additional_tools role is unsupported")
			}
			additionalTools = append(additionalTools, item.Tools...)
		default:
			return nil, nil, unsupportedProtocolField("Responses", "input."+item.Type)
		}
	}
	return messages, additionalTools, nil
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

func responsesMessageContent(raw json.RawMessage) (json.RawMessage, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if text == "" {
			return nil, invalidProtocolRequest("Responses", "message content must not be empty")
		}
		return rawString(text), nil
	}
	var parts []responsesContentPart
	if err := decodeStrict(raw, &parts); err != nil || len(parts) == 0 || len(parts) > maxMessageParts {
		return nil, invalidProtocolRequest("Responses", "message content is unsupported")
	}
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		if (part.Type != "input_text" && part.Type != "output_text" && part.Type != "text") || part.Text == "" {
			return nil, unsupportedProtocolField("Responses", "input.content."+part.Type)
		}
		texts = append(texts, part.Text)
	}
	return joinTextParts(texts), nil
}

func responsesTextContent(raw json.RawMessage, allowPlain bool) (string, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if !allowPlain && text == "" {
			return "", errors.New("empty text")
		}
		return text, nil
	}
	var parts []responsesContentPart
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

func responsesToolChoice(raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	var choice string
	if json.Unmarshal(raw, &choice) == nil {
		if choice == "auto" || choice == "none" || choice == "required" {
			return rawString(choice), nil
		}
		return nil, invalidProtocolRequest("Responses", "tool_choice is unsupported")
	}
	var named struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if decodeStrict(raw, &named) != nil || named.Type != "function" || !validToolName(named.Name) {
		return nil, invalidProtocolRequest("Responses", "tool_choice is unsupported")
	}
	value := namedToolChoice{Type: "function"}
	value.Function.Name = named.Name
	data, _ := json.Marshal(value)
	return data, nil
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
