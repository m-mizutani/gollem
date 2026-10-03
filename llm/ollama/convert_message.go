package ollama

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gollem-dev/gollem"
	"github.com/gollem-dev/gollem/internal/convert"
	"github.com/gollem-dev/gollem/internal/jsonutil"
	"github.com/m-mizutani/goerr/v2"
)

const (
	roleSystem    = "system"
	roleUser      = "user"
	roleAssistant = "assistant"
	roleTool      = "tool"

	// rawArgumentsKey and rawContentKey hold a value that is not a JSON object
	// when it is converted to the common history format.
	rawArgumentsKey = "arguments"
	rawContentKey   = "content"
)

// convertInputs converts gollem inputs into Ollama messages. Consecutive Text
// and Image inputs form one user message, because an Ollama message has a
// single content string: the texts are joined with "\n" and the images are
// listed in order.
func convertInputs(input []gollem.Input) ([]message, error) {
	var result []message
	var user *message

	flushUser := func() {
		if user != nil {
			result = append(result, *user)
			user = nil
		}
	}
	userMessage := func() *message {
		if user == nil {
			user = &message{Role: roleUser}
		}
		return user
	}

	for _, in := range input {
		switch v := in.(type) {
		case gollem.Text:
			m := userMessage()
			m.Content = joinText(m.Content, string(v))

		case gollem.Image:
			m := userMessage()
			m.Images = append(m.Images, v.Base64())

		case gollem.FunctionResponse:
			flushUser()
			msg, err := functionResponseMessage(v)
			if err != nil {
				return nil, err
			}
			result = append(result, msg)

		case gollem.PDF:
			return nil, goerr.Wrap(gollem.ErrInvalidParameter, "PDF input is not supported by Ollama")

		default:
			return nil, goerr.Wrap(gollem.ErrInvalidParameter, "unsupported input type",
				goerr.V("type", fmt.Sprintf("%T", in)))
		}
	}
	flushUser()

	return result, nil
}

func functionResponseMessage(v gollem.FunctionResponse) (message, error) {
	content := ""
	if v.Error != nil {
		content = fmt.Sprintf("Error message: %+v", v.Error)
	} else {
		data, err := json.Marshal(v.Data)
		if err != nil {
			return message{}, goerr.Wrap(err, "failed to marshal function response",
				goerr.V("name", v.Name), goerr.V("id", v.ID))
		}
		content = string(data)
	}

	return message{
		Role:       roleTool,
		Content:    content,
		ToolName:   v.Name,
		ToolCallID: v.ID,
	}, nil
}

func joinText(current, text string) string {
	if current == "" {
		return text
	}
	return current + "\n" + text
}

// toMessages converts a gollem.History into Ollama messages.
func toMessages(h *gollem.History) ([]message, error) {
	if h == nil || len(h.Messages) == 0 {
		return []message{}, nil
	}

	result := make([]message, 0, len(h.Messages))
	for i, msg := range h.Messages {
		converted, err := convertMessage(msg)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert history message", goerr.V("index", i))
		}
		result = append(result, converted...)
	}
	return result, nil
}

// convertMessage converts one common message. Tool responses become separate
// tool messages placed after the message that holds the other contents.
func convertMessage(msg gollem.Message) ([]message, error) {
	main := message{Role: ollamaRole(msg.Role)}
	var toolMessages []message

	for _, content := range msg.Contents {
		switch content.Type {
		case gollem.MessageContentTypeText:
			text, err := content.GetTextContent()
			if err != nil {
				return nil, goerr.Wrap(err, "failed to get text content")
			}
			main.Content = joinText(main.Content, text.Text)

		case gollem.MessageContentTypeThinking:
			thinking, err := content.GetThinkingContent()
			if err != nil {
				return nil, goerr.Wrap(err, "failed to get thinking content")
			}
			main.Thinking += thinking.Text

		case gollem.MessageContentTypeImage:
			img, err := content.GetImageContent()
			if err != nil {
				return nil, goerr.Wrap(err, "failed to get image content")
			}
			if len(img.Data) == 0 {
				return nil, goerr.Wrap(gollem.ErrInvalidParameter,
					"image given only by URL is not supported by Ollama", goerr.V("url", img.URL))
			}
			main.Images = append(main.Images, base64.StdEncoding.EncodeToString(img.Data))

		case gollem.MessageContentTypePDF:
			return nil, goerr.Wrap(gollem.ErrInvalidParameter, "PDF input is not supported by Ollama")

		case gollem.MessageContentTypeToolCall:
			call, err := content.GetToolCallContent()
			if err != nil {
				return nil, goerr.Wrap(err, "failed to get tool call content")
			}
			args, err := marshalArguments(call.Arguments)
			if err != nil {
				return nil, goerr.Wrap(err, "failed to marshal tool call arguments",
					goerr.V("name", call.Name), goerr.V("id", call.ID))
			}
			main.ToolCalls = append(main.ToolCalls, toolCall{
				ID: call.ID,
				Function: toolCallFunction{
					Index:     len(main.ToolCalls),
					Name:      call.Name,
					Arguments: args,
				},
			})

		case gollem.MessageContentTypeToolResponse:
			resp, err := content.GetToolResponseContent()
			if err != nil {
				return nil, goerr.Wrap(err, "failed to get tool response content")
			}
			data, err := json.Marshal(resp.Response)
			if err != nil {
				return nil, goerr.Wrap(err, "failed to marshal tool response",
					goerr.V("name", resp.Name), goerr.V("id", resp.ToolCallID))
			}
			toolMessages = append(toolMessages, message{
				Role:       roleTool,
				Content:    string(data),
				ToolName:   resp.Name,
				ToolCallID: resp.ToolCallID,
			})

		default:
			return nil, goerr.Wrap(gollem.ErrInvalidHistoryData, "unknown message content type",
				goerr.V("type", content.Type))
		}
	}

	hasMain := main.Content != "" || main.Thinking != "" || len(main.Images) > 0 || len(main.ToolCalls) > 0
	result := make([]message, 0, 1+len(toolMessages))
	if hasMain || len(toolMessages) == 0 {
		result = append(result, main)
	}
	return append(result, toolMessages...), nil
}

func marshalArguments(args map[string]any) (json.RawMessage, error) {
	if args == nil {
		return json.RawMessage("{}"), nil
	}
	data, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func ollamaRole(role gollem.MessageRole) string {
	switch role {
	case gollem.RoleSystem:
		return roleSystem
	case gollem.RoleAssistant:
		return roleAssistant
	case gollem.RoleTool:
		return roleTool
	default:
		return roleUser
	}
}

// newHistory converts Ollama messages into a gollem.History.
func newHistory(messages []message) (*gollem.History, error) {
	result := make([]gollem.Message, 0, len(messages))
	for i, msg := range messages {
		converted, err := convertToCommon(msg)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert Ollama message", goerr.V("index", i))
		}
		result = append(result, converted)
	}

	return &gollem.History{
		LLType:   gollem.LLMTypeOllama,
		Version:  gollem.HistoryVersion,
		Messages: result,
	}, nil
}

func convertToCommon(msg message) (gollem.Message, error) {
	var contents []gollem.MessageContent
	add := func(c gollem.MessageContent, err error) error {
		if err != nil {
			return err
		}
		contents = append(contents, c)
		return nil
	}

	if msg.Role == roleTool {
		response, err := jsonutil.DecodeObject([]byte(msg.Content))
		if err != nil || response == nil {
			// Error results are sent as plain text ("Error message: ..."), so
			// a non-object content is kept as text rather than rejected.
			response = map[string]any{rawContentKey: msg.Content}
		}
		if err := add(gollem.NewToolResponseContent(msg.ToolCallID, msg.ToolName, response, false)); err != nil {
			return gollem.Message{}, err
		}
		return gollem.Message{Role: gollem.RoleTool, Contents: contents}, nil
	}

	if msg.Thinking != "" {
		if err := add(gollem.NewThinkingContent(msg.Thinking)); err != nil {
			return gollem.Message{}, err
		}
	}
	if msg.Content != "" {
		if err := add(gollem.NewTextContent(msg.Content)); err != nil {
			return gollem.Message{}, err
		}
	}
	for i, img := range msg.Images {
		data, err := base64.StdEncoding.DecodeString(img)
		if err != nil {
			return gollem.Message{}, goerr.Wrap(err, "failed to decode base64 image", goerr.V("index", i))
		}
		if err := add(gollem.NewImageContent(http.DetectContentType(data), data, "", "")); err != nil {
			return gollem.Message{}, err
		}
	}
	for _, call := range msg.ToolCalls {
		args, err := decodeArguments(call.Function.Arguments)
		if err != nil {
			// History conversion must not lose a call the model made, so an
			// argument value that is not an object is kept verbatim.
			args = map[string]any{rawArgumentsKey: string(call.Function.Arguments)}
		}
		if err := add(gollem.NewToolCallContent(call.ID, call.Function.Name, args)); err != nil {
			return gollem.Message{}, err
		}
	}

	return gollem.Message{
		Role:     convert.ConvertRoleToCommon(msg.Role),
		Contents: contents,
	}, nil
}

// decodeArguments decodes tool call arguments. An absent or null value is an
// empty object; any other value that is not a JSON object is an error.
func decodeArguments(raw json.RawMessage) (map[string]any, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return map[string]any{}, nil
	}
	args, err := jsonutil.DecodeObject(trimmed)
	if err != nil {
		return nil, goerr.Wrap(err, "tool call arguments are not a JSON object",
			goerr.V("arguments", truncateForLog(string(trimmed))))
	}
	return args, nil
}

// convertResponseMessage extracts the function calls from an assistant
// message returned by the server. A call without an ID gets one from
// convert.GenerateToolCallID, and the returned message carries the same ID so
// the history matches the IDs given to the caller.
func convertResponseMessage(msg message) ([]*gollem.FunctionCall, message, error) {
	assistant := msg
	assistant.Role = roleAssistant
	if len(msg.ToolCalls) == 0 {
		return nil, assistant, nil
	}

	assistant.ToolCalls = make([]toolCall, len(msg.ToolCalls))
	calls := make([]*gollem.FunctionCall, 0, len(msg.ToolCalls))
	for i, call := range msg.ToolCalls {
		args, err := decodeArguments(call.Function.Arguments)
		if err != nil {
			return nil, message{}, goerr.Wrap(err, "failed to decode tool call arguments",
				goerr.V("name", call.Function.Name))
		}
		if call.ID == "" {
			call.ID = convert.GenerateToolCallID(call.Function.Name, i)
		}
		call.Function.Index = i
		assistant.ToolCalls[i] = call
		calls = append(calls, &gollem.FunctionCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: args,
		})
	}
	return calls, assistant, nil
}

// truncateForLog shortens s so a large model output does not flood an error
// value.
func truncateForLog(s string) string {
	const limit = 256
	if len(s) <= limit {
		return s
	}
	return strings.ToValidUTF8(s[:limit], "") + "..."
}
