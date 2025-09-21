package gollem

import (
	"github.com/m-mizutani/goerr/v2"
	"google.golang.org/genai"
)

// convertGeminiToMessages converts Gemini contents to common Message format
func convertGeminiToMessages(contents []*genai.Content) ([]Message, error) {
	if len(contents) == 0 {
		return []Message{}, nil
	}

	result := make([]Message, 0, len(contents))

	for _, content := range contents {
		msg, err := convertGeminiContent(content)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert Gemini content")
		}
		result = append(result, msg)
	}

	return result, nil
}

// convertGeminiContent converts a single Gemini content to Message
func convertGeminiContent(content *genai.Content) (Message, error) {
	contents := make([]MessageContent, 0, len(content.Parts))

	for _, part := range content.Parts {
		msgContent, err := convertGeminiPart(part)
		if err != nil {
			return Message{}, goerr.Wrap(err, "failed to convert Gemini part")
		}
		contents = append(contents, msgContent)
	}

	// Convert role
	role := convertRoleToCommon(content.Role)
	if content.Role == "model" {
		role = RoleModel // Preserve model role for round-trip
	}

	return Message{
		Role:     role,
		Contents: contents,
	}, nil
}

// convertGeminiPart converts a Gemini part to MessageContent
func convertGeminiPart(part *genai.Part) (MessageContent, error) {
	// Text part
	if part.Text != "" {
		return NewTextContent(part.Text)
	}

	// Inline data (image)
	if part.InlineData != nil {
		return NewImageContent(
			part.InlineData.MIMEType,
			part.InlineData.Data,
			"",
			"",
		)
	}

	// File data
	if part.FileData != nil {
		// Gemini uses file URIs, store as URL
		return NewImageContent(
			part.FileData.MIMEType,
			nil,
			part.FileData.FileURI,
			"",
		)
	}

	// Function call
	if part.FunctionCall != nil {
		return NewToolCallContent(
			generateToolCallID(part.FunctionCall.Name, 0),
			part.FunctionCall.Name,
			part.FunctionCall.Args,
		)
	}

	// Function response
	if part.FunctionResponse != nil {
		return NewToolResponseContent(
			generateToolCallID(part.FunctionResponse.Name, 0),
			part.FunctionResponse.Name,
			part.FunctionResponse.Response,
			false,
		)
	}

	return MessageContent{}, goerr.Wrap(ErrUnsupportedContentType, "unknown Gemini part type")
}

// convertMessagesToGemini converts common Messages to Gemini format
func convertMessagesToGemini(messages []Message) ([]*genai.Content, error) {
	if len(messages) == 0 {
		return []*genai.Content{}, nil
	}

	// Handle system messages by merging into first user message
	messages = mergeSystemIntoFirstUser(messages)

	result := make([]*genai.Content, 0, len(messages))

	for _, msg := range messages {
		// Skip system messages as they've been merged
		if msg.Role == RoleSystem {
			continue
		}

		geminiContent, err := convertMessageToGemini(msg)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert message to Gemini format")
		}
		result = append(result, geminiContent)
	}

	return result, nil
}

// convertMessageToGemini converts a single Message to Gemini format
func convertMessageToGemini(msg Message) (*genai.Content, error) {
	// Convert role
	role := ""
	switch msg.Role {
	case RoleUser:
		role = "user"
	case RoleAssistant:
		role = "model"
	case RoleModel:
		role = "model"
	case RoleTool, RoleFunction:
		// Tool/function responses are handled as function_response parts
		role = "user"
	default:
		role = "user"
	}

	// Convert contents
	parts := make([]*genai.Part, 0, len(msg.Contents))
	for _, content := range msg.Contents {
		part, err := convertContentToGemini(content)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert content to Gemini format")
		}
		parts = append(parts, part)
	}

	return &genai.Content{
		Role:  role,
		Parts: parts,
	}, nil
}

// convertContentToGemini converts MessageContent to Gemini part
func convertContentToGemini(content MessageContent) (*genai.Part, error) {
	switch content.Type {
	case MessageContentTypeText:
		textContent, err := content.GetTextContent()
		if err != nil {
			return nil, err
		}
		return &genai.Part{Text: textContent.Text}, nil

	case MessageContentTypeImage:
		imgContent, err := content.GetImageContent()
		if err != nil {
			return nil, err
		}
		if len(imgContent.Data) > 0 {
			// Inline data
			return &genai.Part{
				InlineData: &genai.Blob{
					MIMEType: imgContent.MediaType,
					Data:     imgContent.Data,
				},
			}, nil
		} else if imgContent.URL != "" {
			// File URI
			return &genai.Part{
				FileData: &genai.FileData{
					MIMEType: imgContent.MediaType,
					FileURI:  imgContent.URL,
				},
			}, nil
		}
		return nil, goerr.Wrap(ErrInvalidMessageFormat, "image has neither data nor URL")

	case MessageContentTypeToolCall:
		toolCall, err := content.GetToolCallContent()
		if err != nil {
			return nil, err
		}
		return &genai.Part{
			FunctionCall: &genai.FunctionCall{
				Name: toolCall.Name,
				Args: toolCall.Arguments,
			},
		}, nil

	case MessageContentTypeToolResponse:
		toolResp, err := content.GetToolResponseContent()
		if err != nil {
			return nil, err
		}
		return &genai.Part{
			FunctionResponse: &genai.FunctionResponse{
				Name:     toolResp.Name,
				Response: toolResp.Response,
			},
		}, nil

	case MessageContentTypeFunctionCall:
		// Convert legacy function call to tool call
		funcCall, err := content.GetFunctionCallContent()
		if err != nil {
			return nil, err
		}
		args, _ := parseJSONArguments(funcCall.Arguments)
		return &genai.Part{
			FunctionCall: &genai.FunctionCall{
				Name: funcCall.Name,
				Args: args,
			},
		}, nil

	case MessageContentTypeFunctionResponse:
		// Convert legacy function response to tool response
		funcResp, err := content.GetFunctionResponseContent()
		if err != nil {
			return nil, err
		}
		return &genai.Part{
			FunctionResponse: &genai.FunctionResponse{
				Name:     funcResp.Name,
				Response: map[string]interface{}{"content": funcResp.Content},
			},
		}, nil

	default:
		return nil, goerr.Wrap(ErrUnsupportedContentType, "unsupported content type for Gemini", goerr.Value("type", content.Type))
	}
}
