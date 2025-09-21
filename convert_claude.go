package gollem

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/m-mizutani/goerr/v2"
)

// convertClaudeToMessages converts Claude messages to common Message format
func convertClaudeToMessages(messages []anthropic.MessageParam) ([]Message, error) {
	if len(messages) == 0 {
		return []Message{}, nil
	}

	result := make([]Message, 0, len(messages))

	for _, msg := range messages {
		contents := make([]MessageContent, 0, len(msg.Content))

		for _, block := range msg.Content {
			content, err := convertClaudeContentBlock(block)
			if err != nil {
				return nil, goerr.Wrap(err, "failed to convert Claude content block")
			}
			contents = append(contents, content)
		}

		result = append(result, Message{
			Role:     convertRoleToCommon(string(msg.Role)),
			Contents: contents,
		})
	}

	return result, nil
}

// convertClaudeContentBlock converts a single Claude content block to MessageContent
func convertClaudeContentBlock(block anthropic.ContentBlockParamUnion) (MessageContent, error) {
	// Handle text blocks
	if block.OfText != nil {
		return NewTextContent(block.OfText.Text)
	}

	// Handle image blocks
	if block.OfImage != nil {
		if block.OfImage.Source.OfBase64 != nil {
			// Decode the Base64 string to raw bytes
			decodedData, err := base64.StdEncoding.DecodeString(block.OfImage.Source.OfBase64.Data)
			if err != nil {
				// If decoding fails, treat it as raw data
				// This allows handling of both valid Base64 and raw strings
				decodedData = []byte(block.OfImage.Source.OfBase64.Data)
			}
			return NewImageContent(
				string(block.OfImage.Source.OfBase64.MediaType),
				decodedData,
				"",
				"",
			)
		}
		// Handle URL images if supported
		// Note: Claude API primarily uses base64 images
	}

	// Handle tool use blocks
	if block.OfToolUse != nil {
		// Convert input to map if it's not already
		var args map[string]interface{}
		switch v := block.OfToolUse.Input.(type) {
		case map[string]interface{}:
			args = v
		case string:
			// Try to parse as JSON
			if err := json.Unmarshal([]byte(v), &args); err != nil {
				args = map[string]interface{}{"input": v}
			}
		default:
			// Convert to JSON then back to map
			data, _ := json.Marshal(v)
			_ = json.Unmarshal(data, &args)
		}

		return NewToolCallContent(
			block.OfToolUse.ID,
			block.OfToolUse.Name,
			args,
		)
	}

	// Handle tool result blocks
	if block.OfToolResult != nil {
		// Extract text content from tool result
		responseText := ""
		if len(block.OfToolResult.Content) > 0 && block.OfToolResult.Content[0].OfText != nil {
			responseText = block.OfToolResult.Content[0].OfText.Text
		}

		isError := false
		if block.OfToolResult.IsError.Valid() {
			isError = block.OfToolResult.IsError.Value
		}

		return NewToolResponseContent(
			block.OfToolResult.ToolUseID,
			"", // Claude doesn't include tool name in response
			map[string]interface{}{"content": responseText},
			isError,
		)
	}

	return MessageContent{}, goerr.Wrap(ErrUnsupportedContentType, "unknown Claude content block type")
}

// convertMessagesToClaude converts common Messages to Claude format
func convertMessagesToClaude(messages []Message) ([]anthropic.MessageParam, error) {
	if len(messages) == 0 {
		return []anthropic.MessageParam{}, nil
	}

	// Handle system messages by merging into first user message
	messages = mergeSystemIntoFirstUser(messages)

	result := make([]anthropic.MessageParam, 0, len(messages))

	for _, msg := range messages {
		// Skip system messages as they've been merged
		if msg.Role == RoleSystem {
			continue
		}
		// Skip empty messages
		if len(msg.Contents) == 0 {
			continue
		}

		claudeMsg, err := convertMessageToClaude(msg)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert message to Claude format")
		}
		// Skip messages with no content after conversion
		if len(claudeMsg.Content) == 0 {
			continue
		}
		result = append(result, claudeMsg)
	}

	return result, nil
}

// convertMessageToClaude converts a single Message to Claude format
func convertMessageToClaude(msg Message) (anthropic.MessageParam, error) {
	// Convert role
	var role anthropic.MessageParamRole
	switch msg.Role {
	case RoleUser:
		role = anthropic.MessageParamRoleUser
	case RoleAssistant, RoleModel:
		role = anthropic.MessageParamRoleAssistant
	case RoleTool, RoleFunction:
		// Tool/function responses should be in user role with tool_result block
		role = anthropic.MessageParamRoleUser
	default:
		role = anthropic.MessageParamRoleUser
	}

	// Convert contents
	contents := make([]anthropic.ContentBlockParamUnion, 0, len(msg.Contents))
	for _, content := range msg.Contents {
		claudeContent, err := convertContentToClaude(content, msg.Role)
		if err != nil {
			// Skip unsupported content types instead of failing completely
			if err == ErrUnsupportedContentType {
				continue
			}
			return anthropic.MessageParam{}, goerr.Wrap(err, "failed to convert content to Claude format")
		}
		contents = append(contents, claudeContent)
	}

	return anthropic.MessageParam{
		Role:    role,
		Content: contents,
	}, nil
}

// convertContentToClaude converts MessageContent to Claude content block
func convertContentToClaude(content MessageContent, messageRole MessageRole) (anthropic.ContentBlockParamUnion, error) {
	_ = messageRole // Currently unused but may be needed for future conversions
	switch content.Type {
	case MessageContentTypeText:
		textContent, err := content.GetTextContent()
		if err != nil {
			return anthropic.ContentBlockParamUnion{}, err
		}
		return anthropic.NewTextBlock(textContent.Text), nil

	case MessageContentTypeImage:
		imgContent, err := content.GetImageContent()
		if err != nil {
			return anthropic.ContentBlockParamUnion{}, err
		}
		// Convert to base64 if we have raw data
		if len(imgContent.Data) > 0 {
			return anthropic.NewImageBlockBase64(imgContent.MediaType, base64.StdEncoding.EncodeToString(imgContent.Data)), nil
		}
		// For URL images, create a text block with the URL reference
		// This maintains the information even though Claude can't directly display the image
		if imgContent.URL != "" {
			imageRef := fmt.Sprintf("[Image: %s]", imgContent.URL)
			if imgContent.Detail != "" {
				imageRef = fmt.Sprintf("[Image (%s): %s]", imgContent.Detail, imgContent.URL)
			}
			return anthropic.NewTextBlock(imageRef), nil
		}
		return anthropic.ContentBlockParamUnion{}, ErrUnsupportedContentType

	case MessageContentTypeToolCall:
		toolCall, err := content.GetToolCallContent()
		if err != nil {
			return anthropic.ContentBlockParamUnion{}, err
		}
		return anthropic.NewToolUseBlock(toolCall.ID, toolCall.Arguments, toolCall.Name), nil

	case MessageContentTypeToolResponse:
		toolResp, err := content.GetToolResponseContent()
		if err != nil {
			return anthropic.ContentBlockParamUnion{}, err
		}
		// Extract content string from response map
		contentStr := ""
		if c, ok := toolResp.Response["content"].(string); ok {
			contentStr = c
		} else {
			// Try to JSON stringify the response
			data, _ := json.Marshal(toolResp.Response)
			contentStr = string(data)
		}

		return anthropic.NewToolResultBlock(toolResp.ToolCallID, contentStr, toolResp.IsError), nil

	case MessageContentTypeFunctionCall:
		// Convert legacy function call to tool call
		funcCall, err := content.GetFunctionCallContent()
		if err != nil {
			return anthropic.ContentBlockParamUnion{}, err
		}
		args, _ := parseJSONArguments(funcCall.Arguments)
		return anthropic.NewToolUseBlock(generateToolCallID(funcCall.Name, 0), args, funcCall.Name), nil

	case MessageContentTypeFunctionResponse:
		// Convert legacy function response to tool result
		funcResp, err := content.GetFunctionResponseContent()
		if err != nil {
			return anthropic.ContentBlockParamUnion{}, err
		}
		// Generate a tool call ID based on function name
		toolCallID := generateToolCallID(funcResp.Name, 0)
		return anthropic.NewToolResultBlock(toolCallID, funcResp.Content, false), nil

	default:
		return anthropic.ContentBlockParamUnion{}, goerr.Wrap(ErrUnsupportedContentType, "unsupported content type for Claude", goerr.Value("type", content.Type))
	}
}
