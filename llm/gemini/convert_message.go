package gemini

import (
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/internal/convert"
	"google.golang.org/genai"
)

// convertGeminiToMessages converts Gemini contents to common Message format
func convertGeminiToMessages(contents []*genai.Content) ([]gollem.Message, error) {
	if len(contents) == 0 {
		return []gollem.Message{}, nil
	}

	result := make([]gollem.Message, 0, len(contents))

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
func convertGeminiContent(content *genai.Content) (gollem.Message, error) {
	contents := make([]gollem.MessageContent, 0, len(content.Parts))

	for _, part := range content.Parts {
		msgContent, err := convertGeminiPart(part)
		if err != nil {
			return gollem.Message{}, goerr.Wrap(err, "failed to convert Gemini part")
		}
		contents = append(contents, msgContent)
	}

	// Convert role - model is always converted to assistant
	role := convert.ConvertRoleToCommon(content.Role)

	return gollem.Message{
		Role:     role,
		Contents: contents,
	}, nil
}

// convertGeminiPart converts a Gemini part to MessageContent
func convertGeminiPart(part *genai.Part) (gollem.MessageContent, error) {
	// Text part
	if part.Text != "" {
		return gollem.NewTextContent(part.Text)
	}

	// Inline data (image or PDF)
	if part.InlineData != nil {
		if part.InlineData.MIMEType == "application/pdf" {
			return gollem.NewPDFContent(part.InlineData.Data, "")
		}
		return gollem.NewImageContent(
			part.InlineData.MIMEType,
			part.InlineData.Data,
			"",
			"",
		)
	}

	// File data
	if part.FileData != nil {
		// Gemini uses file URIs, store as URL
		return gollem.NewImageContent(
			part.FileData.MIMEType,
			nil,
			part.FileData.FileURI,
			"",
		)
	}

	// Function call
	if part.FunctionCall != nil {
		return gollem.NewToolCallContent(
			convert.GenerateToolCallID(part.FunctionCall.Name, 0),
			part.FunctionCall.Name,
			part.FunctionCall.Args,
		)
	}

	// Function response
	if part.FunctionResponse != nil {
		return gollem.NewToolResponseContent(
			convert.GenerateToolCallID(part.FunctionResponse.Name, 0),
			part.FunctionResponse.Name,
			part.FunctionResponse.Response,
			false,
		)
	}

	return gollem.MessageContent{}, goerr.Wrap(convert.ErrUnsupportedContentType, "unknown Gemini part type")
}

// convertMessagesToGemini converts common Messages to Gemini format
func convertMessagesToGemini(messages []gollem.Message) ([]*genai.Content, error) {
	if len(messages) == 0 {
		return []*genai.Content{}, nil
	}

	// Handle system messages by merging into first user message
	messages = convert.MergeSystemIntoFirstUser(messages)

	result := make([]*genai.Content, 0, len(messages))

	for _, msg := range messages {
		// Skip system messages as they've been merged
		if msg.Role == gollem.RoleSystem {
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
func convertMessageToGemini(msg gollem.Message) (*genai.Content, error) {
	// Convert role
	role := ""
	switch msg.Role {
	case gollem.RoleUser:
		role = "user"
	case gollem.RoleAssistant:
		// Assistant is always converted to model for Gemini SDK
		role = "model"
	case gollem.RoleTool:
		// Tool responses go to user role in Gemini
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
func convertContentToGemini(content gollem.MessageContent) (*genai.Part, error) {
	switch content.Type {
	case gollem.MessageContentTypeText:
		textContent, err := content.GetTextContent()
		if err != nil {
			return nil, err
		}
		return &genai.Part{Text: textContent.Text}, nil

	case gollem.MessageContentTypeImage:
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
		return nil, goerr.Wrap(convert.ErrInvalidMessageFormat, "image has neither data nor URL")

	case gollem.MessageContentTypePDF:
		pdfContent, err := content.GetPDFContent()
		if err != nil {
			return nil, err
		}
		if len(pdfContent.Data) > 0 {
			return &genai.Part{
				InlineData: &genai.Blob{
					MIMEType: "application/pdf",
					Data:     pdfContent.Data,
				},
			}, nil
		}
		if pdfContent.URL != "" {
			return &genai.Part{
				FileData: &genai.FileData{
					MIMEType: "application/pdf",
					FileURI:  pdfContent.URL,
				},
			}, nil
		}
		return nil, goerr.Wrap(convert.ErrInvalidMessageFormat, "PDF has neither data nor URL")

	case gollem.MessageContentTypeToolCall:
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

	case gollem.MessageContentTypeToolResponse:
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

	default:
		return nil, goerr.Wrap(convert.ErrUnsupportedContentType, "unsupported content type for Gemini", goerr.Value("type", content.Type))
	}
}

// ToContents converts gollem.History to Gemini contents
func ToContents(h *gollem.History) ([]*genai.Content, error) {
	if h == nil || len(h.Messages) == 0 {
		return []*genai.Content{}, nil
	}
	return convertMessagesToGemini(h.Messages)
}

// NewHistory creates gollem.History from Gemini contents
func NewHistory(contents []*genai.Content) (*gollem.History, error) {
	commonMessages, err := convertGeminiToMessages(contents)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to convert Gemini messages to common format")
	}

	return &gollem.History{
		LLType:   gollem.LLMTypeGemini,
		Version:  gollem.HistoryVersion,
		Messages: commonMessages,
	}, nil
}
