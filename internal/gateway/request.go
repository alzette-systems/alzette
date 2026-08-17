package gateway

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const (
	maxMessages       = 128
	maxMessageParts   = 128
	maxTools          = 128
	maxToolCalls      = 128
	maxToolNameLength = 64
)

// ChatRequest is the deliberately bounded OpenAI Chat Completions request
// subset accepted by the gateway. Routing and provider configuration are not
// represented here: those values always come from the authenticated route.
type ChatRequest struct {
	Model         string          `json:"model"`
	Messages      []Message       `json:"messages"`
	Stream        *bool           `json:"stream,omitempty"`
	StreamOptions *StreamOptions  `json:"stream_options,omitempty"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	MaxTokens     *int            `json:"max_tokens,omitempty"`
	Tools         []Tool          `json:"tools,omitempty"`
	ToolChoice    json.RawMessage `json:"tool_choice,omitempty"`
}

type StreamOptions struct {
	IncludeUsage *bool `json:"include_usage"`
}

type Message struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	ToolCalls  []ToolCall      `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Name       string          `json:"name,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type Tool struct {
	Type     string         `json:"type"`
	Function ToolDefinition `json:"function"`
}

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
	Strict      *bool           `json:"strict,omitempty"`
}

type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type textPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type namedToolChoice struct {
	Type     string `json:"type"`
	Function struct {
		Name string `json:"name"`
	} `json:"function"`
}

func (r ChatRequest) streaming() bool {
	return r.Stream != nil && *r.Stream
}

func (r ChatRequest) validate() *requestError {
	if r.Model == "" || len(r.Model) > 128 {
		return &requestError{http.StatusBadRequest, "invalid_model", "model alias is required"}
	}
	if len(r.Messages) == 0 || len(r.Messages) > maxMessages {
		return &requestError{http.StatusBadRequest, "invalid_messages", "messages must contain between 1 and 128 entries"}
	}
	if validation := validateMessages(r.Messages); validation != nil {
		return validation
	}
	if r.Temperature != nil && (*r.Temperature < 0 || *r.Temperature > 2) {
		return &requestError{http.StatusBadRequest, "invalid_temperature", "temperature must be between 0 and 2"}
	}
	if r.TopP != nil && (*r.TopP < 0 || *r.TopP > 1) {
		return &requestError{http.StatusBadRequest, "invalid_top_p", "top_p must be between 0 and 1"}
	}
	if r.MaxTokens != nil && (*r.MaxTokens < 1 || *r.MaxTokens > 1000000) {
		return &requestError{http.StatusBadRequest, "invalid_max_tokens", "max_tokens is outside the supported range"}
	}
	if r.StreamOptions != nil {
		if !r.streaming() || r.StreamOptions.IncludeUsage == nil {
			return &requestError{http.StatusBadRequest, "invalid_stream_options", "stream_options requires streaming and an include_usage boolean"}
		}
	}
	toolNames, validation := validateTools(r.Tools)
	if validation != nil {
		return validation
	}
	if validation := validateToolChoice(r.ToolChoice, toolNames); validation != nil {
		return validation
	}
	return nil
}

func validateMessages(messages []Message) *requestError {
	outstanding := make(map[string]string)
	seenCalls := make(map[string]struct{})
	for _, message := range messages {
		if message.Role != "tool" && len(outstanding) != 0 {
			return invalidMessages("tool results must immediately follow the assistant tool calls they answer")
		}
		switch message.Role {
		case "system", "user":
			if len(message.ToolCalls) != 0 || message.ToolCallID != "" || message.Name != "" {
				return invalidMessages("message fields are outside the supported text subset")
			}
			if ok, err := validTextContent(message.Content, false, true); err != nil || !ok {
				return invalidMessages("system and user content must be non-empty text")
			}
		case "assistant":
			if message.ToolCallID != "" || message.Name != "" || len(message.ToolCalls) > maxToolCalls {
				return invalidMessages("assistant message fields are outside the supported subset")
			}
			hasText, err := validTextContent(message.Content, len(message.ToolCalls) > 0, true)
			if err != nil || (!hasText && len(message.ToolCalls) == 0) {
				return invalidMessages("assistant content must be text or accompanied by function tool calls")
			}
			for _, call := range message.ToolCalls {
				if call.Type != "function" || !validToolName(call.Function.Name) || !validToolCallID(call.ID) {
					return invalidMessages("assistant tool calls must be named function calls")
				}
				if _, exists := seenCalls[call.ID]; exists {
					return invalidMessages("assistant tool call IDs must be unique")
				}
				if !validJSONObjectString(call.Function.Arguments) {
					return invalidMessages("assistant tool call arguments must be a JSON object string")
				}
				seenCalls[call.ID] = struct{}{}
				outstanding[call.ID] = call.Function.Name
			}
		case "tool":
			if len(message.ToolCalls) != 0 || !validToolCallID(message.ToolCallID) {
				return invalidMessages("tool messages require a valid tool_call_id")
			}
			expectedName, exists := outstanding[message.ToolCallID]
			if !exists {
				return invalidMessages("tool messages must answer a preceding assistant tool call")
			}
			if message.Name != "" && (message.Name != expectedName || !validToolName(message.Name)) {
				return invalidMessages("tool result name does not match its assistant tool call")
			}
			if ok, err := validTextContent(message.Content, false, false); err != nil || !ok {
				return invalidMessages("tool result content must be a non-empty string")
			}
			delete(outstanding, message.ToolCallID)
		default:
			return invalidMessages("message role is outside the supported subset")
		}
	}
	if len(outstanding) != 0 {
		return invalidMessages("every assistant tool call must have a following tool result")
	}
	return nil
}

func validTextContent(raw json.RawMessage, allowNull, allowParts bool) (bool, error) {
	if len(raw) == 0 {
		return false, fmt.Errorf("missing content")
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if allowNull {
			return false, nil
		}
		return false, fmt.Errorf("null content is not allowed")
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text != "", nil
	}
	if !allowParts {
		return false, fmt.Errorf("content parts are unsupported")
	}
	var parts []textPart
	if err := decodeStrict(raw, &parts); err != nil || len(parts) == 0 || len(parts) > maxMessageParts {
		return false, fmt.Errorf("invalid text content parts")
	}
	hasText := false
	for _, part := range parts {
		if part.Type != "text" || part.Text == "" {
			return false, fmt.Errorf("unsupported content part")
		}
		hasText = true
	}
	return hasText, nil
}

func validateTools(tools []Tool) (map[string]struct{}, *requestError) {
	names := make(map[string]struct{}, len(tools))
	if len(tools) > maxTools {
		return nil, invalidTools("tools must contain at most 128 function definitions")
	}
	for _, tool := range tools {
		if tool.Type != "function" || !validToolName(tool.Function.Name) {
			return nil, invalidTools("tools must be named function definitions")
		}
		if len(tool.Function.Description) > 8192 {
			return nil, invalidTools("tool descriptions exceed the supported limit")
		}
		if _, exists := names[tool.Function.Name]; exists {
			return nil, invalidTools("tool names must be unique")
		}
		if err := validateToolParameters(tool.Function.Parameters); err != nil {
			return nil, invalidTools("tool parameters must be a supported object JSON Schema")
		}
		names[tool.Function.Name] = struct{}{}
	}
	return names, nil
}

func validateToolParameters(raw json.RawMessage) error {
	var schema map[string]interface{}
	if err := decodeJSONValue(raw, &schema); err != nil || schema == nil {
		return fmt.Errorf("invalid schema")
	}
	if schemaType, ok := schema["type"].(string); !ok || schemaType != "object" {
		return fmt.Errorf("schema type must be object")
	}
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("schema properties must be an object")
	}
	if requiredValue, exists := schema["required"]; exists {
		required, ok := requiredValue.([]interface{})
		if !ok {
			return fmt.Errorf("schema required must be an array")
		}
		seen := make(map[string]struct{}, len(required))
		for _, value := range required {
			name, ok := value.(string)
			if !ok || name == "" {
				return fmt.Errorf("schema required entries must be names")
			}
			if _, exists := properties[name]; !exists {
				return fmt.Errorf("schema required entry is not a property")
			}
			if _, exists := seen[name]; exists {
				return fmt.Errorf("schema required entries must be unique")
			}
			seen[name] = struct{}{}
		}
	}
	return validateJSONDepth(schema, 0)
}

func validateJSONDepth(value interface{}, depth int) error {
	if depth > 32 {
		return fmt.Errorf("JSON nesting exceeds the supported limit")
	}
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			if key == "" || len(key) > 256 {
				return fmt.Errorf("invalid JSON object key")
			}
			if err := validateJSONDepth(child, depth+1); err != nil {
				return err
			}
		}
	case []interface{}:
		for _, child := range typed {
			if err := validateJSONDepth(child, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateToolChoice(raw json.RawMessage, tools map[string]struct{}) *requestError {
	if len(raw) == 0 {
		return nil
	}
	var choice string
	if err := json.Unmarshal(raw, &choice); err == nil {
		if choice != "auto" && choice != "none" && choice != "required" {
			return invalidToolChoice()
		}
		if choice != "none" && len(tools) == 0 {
			return invalidToolChoice()
		}
		return nil
	}
	var named namedToolChoice
	if err := decodeStrict(raw, &named); err != nil || named.Type != "function" || !validToolName(named.Function.Name) {
		return invalidToolChoice()
	}
	if _, exists := tools[named.Function.Name]; !exists {
		return invalidToolChoice()
	}
	return nil
}

func validToolName(value string) bool {
	if value == "" || len(value) > maxToolNameLength {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func validToolCallID(value string) bool {
	return value != "" && len(value) <= 200 && safeMetadata(value, "-")
}

func validJSONObjectString(value string) bool {
	var object map[string]interface{}
	return decodeJSONValue([]byte(value), &object) == nil && object != nil
}

func decodeStrict(raw []byte, destination interface{}) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err == nil {
		return fmt.Errorf("multiple JSON values")
	} else if !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func decodeJSONValue(raw []byte, destination interface{}) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err == nil {
		return fmt.Errorf("multiple JSON values")
	} else if !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func invalidMessages(message string) *requestError {
	return &requestError{http.StatusBadRequest, "invalid_messages", message}
}

func invalidTools(message string) *requestError {
	return &requestError{http.StatusBadRequest, "invalid_tools", message}
}

func invalidToolChoice() *requestError {
	return &requestError{http.StatusBadRequest, "invalid_tool_choice", "tool_choice must select a declared supported function tool"}
}
