package gollem

import (
	"encoding/json"

	"github.com/m-mizutani/goerr/v2"
	"github.com/sashabaranov/go-openai"
)

// convertOpenAIToMessages converts OpenAI messages to common Message format
func convertOpenAIToMessages(messages []openai.ChatCompletionMessage) ([]Message, error) {
	if len(messages) == 0 {
		return []Message{}, nil
	}

	result := make([]Message, 0, len(messages))

	for _, msg := range messages {
		commonMsg, err := convertOpenAIMessage(msg)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert OpenAI message")
		}
		result = append(result, commonMsg)
	}

	return result, nil
}

// convertOpenAIMessage converts a single OpenAI message to common format
func convertOpenAIMessage(msg openai.ChatCompletionMessage) (Message, error) {
	contents := make([]MessageContent, 0)

	// Handle different content types
	if msg.Content != "" {
		// Simple text content
		content, err := NewTextContent(msg.Content)
		if err != nil {
			return Message{}, err
		}
		contents = append(contents, content)
	}

	// Handle MultiContent (for messages with text and images)
	if msg.MultiContent != nil {
		for _, part := range msg.MultiContent {
			if part.Type == "text" {
				content, err := NewTextContent(part.Text)
				if err != nil {
					return Message{}, err
				}
				contents = append(contents, content)
			} else if part.Type == "image_url" && part.ImageURL != nil {
				// Extract image data from URL if it's a data URL
				var imageData []byte
				url := part.ImageURL.URL
				detail := string(part.ImageURL.Detail)

				// TODO: Handle data URLs and extract base64 data
				content, err := NewImageContent("", imageData, url, detail)
				if err != nil {
					return Message{}, err
				}
				contents = append(contents, content)
			}
		}
	}

	// Handle tool calls
	if msg.ToolCalls != nil {
		for _, toolCall := range msg.ToolCalls {
			if toolCall.Function.Name != "" {
				args, err := parseJSONArguments(toolCall.Function.Arguments)
				if err != nil {
					// Use raw string if parsing fails
					args = map[string]interface{}{"arguments": toolCall.Function.Arguments}
				}
				content, err := NewToolCallContent(toolCall.ID, toolCall.Function.Name, args)
				if err != nil {
					return Message{}, err
				}
				contents = append(contents, content)
			}
		}
	}

	// Handle legacy function call
	if msg.FunctionCall != nil {
		content, err := NewFunctionCallContent(msg.FunctionCall.Name, msg.FunctionCall.Arguments)
		if err != nil {
			return Message{}, err
		}
		contents = append(contents, content)
	}

	// Handle tool responses (tool role messages)
	if msg.Role == "tool" && msg.ToolCallID != "" {
		// Parse content as response
		var response map[string]interface{}
		if err := json.Unmarshal([]byte(msg.Content), &response); err != nil {
			// If not JSON, wrap in a response object
			response = map[string]interface{}{"content": msg.Content}
		}
		content, err := NewToolResponseContent(msg.ToolCallID, msg.Name, response, false)
		if err != nil {
			return Message{}, err
		}
		contents = append(contents, content)
	}

	// Handle legacy function responses
	if msg.Role == "function" {
		content, err := NewFunctionResponseContent(msg.Name, msg.Content)
		if err != nil {
			return Message{}, err
		}
		contents = append(contents, content)
	}

	return Message{
		Role:     convertRoleToCommon(msg.Role),
		Contents: contents,
		Name:     msg.Name,
	}, nil
}

// convertMessagesToOpenAI converts common Messages to OpenAI format
func convertMessagesToOpenAI(messages []Message) ([]openai.ChatCompletionMessage, error) {
	if len(messages) == 0 {
		return []openai.ChatCompletionMessage{}, nil
	}

	result := make([]openai.ChatCompletionMessage, 0, len(messages))

	for _, msg := range messages {
		openaiMsgs, err := convertMessageToOpenAI(msg)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert message to OpenAI format")
		}
		result = append(result, openaiMsgs...)
	}

	return result, nil
}

// convertMessageToOpenAI converts a single Message to OpenAI format
// Returns multiple messages as tool responses need separate messages
func convertMessageToOpenAI(msg Message) ([]openai.ChatCompletionMessage, error) {
	// Handle role conversion
	role := ""
	switch msg.Role {
	case RoleSystem:
		role = "system"
	case RoleUser:
		role = "user"
	case RoleAssistant, RoleModel:
		role = "assistant"
	case RoleTool:
		role = "tool"
	case RoleFunction:
		role = "function"
	default:
		role = string(msg.Role)
	}

	// Check if this is a simple text message
	if len(msg.Contents) == 1 && msg.Contents[0].Type == MessageContentTypeText {
		textContent, err := msg.Contents[0].GetTextContent()
		if err != nil {
			return nil, err
		}
		return []openai.ChatCompletionMessage{{
			Role:    role,
			Content: textContent.Text,
			Name:    msg.Name,
		}}, nil
	}

	// Handle complex messages
	var textParts []openai.ChatMessagePart
	var toolCalls []openai.ToolCall
	var functionCall *openai.FunctionCall
	var toolResponses []openai.ChatCompletionMessage

	for _, content := range msg.Contents {
		switch content.Type {
		case MessageContentTypeText:
			textContent, err := content.GetTextContent()
			if err != nil {
				return nil, goerr.Wrap(err, "failed to get text content")
			}
			textParts = append(textParts, openai.ChatMessagePart{
				Type: "text",
				Text: textContent.Text,
			})

		case MessageContentTypeImage:
			imgContent, err := content.GetImageContent()
			if err != nil {
				return nil, goerr.Wrap(err, "failed to get image content")
			}
			imageURL := imgContent.URL
			if len(imgContent.Data) > 0 {
				// Convert to data URL
				imageURL = "data:" + imgContent.MediaType + ";base64," + string(imgContent.Data)
			}
			textParts = append(textParts, openai.ChatMessagePart{
				Type: "image_url",
				ImageURL: &openai.ChatMessageImageURL{
					URL:    imageURL,
					Detail: openai.ImageURLDetail(imgContent.Detail),
				},
			})

		case MessageContentTypeToolCall:
			toolCall, err := content.GetToolCallContent()
			if err != nil {
				return nil, goerr.Wrap(err, "failed to get tool call content")
			}
			args, err := stringifyJSONArguments(toolCall.Arguments)
			if err != nil {
				args = "{}"
			}
			toolCalls = append(toolCalls, openai.ToolCall{
				ID:   toolCall.ID,
				Type: "function",
				Function: openai.FunctionCall{
					Name:      toolCall.Name,
					Arguments: args,
				},
			})

		case MessageContentTypeToolResponse:
			toolResp, err := content.GetToolResponseContent()
			if err != nil {
				return nil, goerr.Wrap(err, "failed to get tool response content")
			}
			respStr, err := stringifyJSONArguments(toolResp.Response)
			if err != nil {
				respStr = "{}"
			}
			toolResponses = append(toolResponses, openai.ChatCompletionMessage{
				Role:       "tool",
				Content:    respStr,
				Name:       toolResp.Name,
				ToolCallID: toolResp.ToolCallID,
			})

		case MessageContentTypeFunctionCall:
			funcCall, err := content.GetFunctionCallContent()
			if err != nil {
				return nil, goerr.Wrap(err, "failed to get function call content")
			}
			functionCall = &openai.FunctionCall{
				Name:      funcCall.Name,
				Arguments: funcCall.Arguments,
			}

		case MessageContentTypeFunctionResponse:
			funcResp, err := content.GetFunctionResponseContent()
			if err != nil {
				return nil, goerr.Wrap(err, "failed to get function response content")
			}
			toolResponses = append(toolResponses, openai.ChatCompletionMessage{
				Role:    "function",
				Content: funcResp.Content,
				Name:    funcResp.Name,
			})
		}
	}

	// Build the result messages
	var result []openai.ChatCompletionMessage

	// Add main message if it has content or tool calls
	if len(textParts) > 0 || len(toolCalls) > 0 || functionCall != nil {
		mainMsg := openai.ChatCompletionMessage{
			Role: role,
			Name: msg.Name,
		}

		// Set content based on what we have
		if len(textParts) == 1 && textParts[0].Type == "text" {
			// Simple text content
			mainMsg.Content = textParts[0].Text
		} else if len(textParts) > 0 {
			// Multi-part content
			mainMsg.MultiContent = textParts
		}

		// Add tool calls
		if len(toolCalls) > 0 {
			mainMsg.ToolCalls = toolCalls
		}

		// Add function call
		if functionCall != nil {
			mainMsg.FunctionCall = functionCall
		}

		result = append(result, mainMsg)
	}

	// Add tool response messages (they must be separate messages)
	result = append(result, toolResponses...)

	// If we have no messages, create an empty text message
	if len(result) == 0 {
		result = append(result, openai.ChatCompletionMessage{
			Role:    role,
			Content: "",
			Name:    msg.Name,
		})
	}

	return result, nil
}
