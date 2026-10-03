package ollama_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gollem-dev/gollem"
	"github.com/gollem-dev/gollem/llm/ollama"
	"github.com/m-mizutani/gt"
)

type noParamTool struct{}

func (noParamTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{Name: "now", Description: "Current time"}
}

func (noParamTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return nil, nil
}

type nestedTool struct{}

func (nestedTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name: "order",
		Parameters: map[string]*gollem.Parameter{
			"items": {
				Type: gollem.TypeArray,
				Items: &gollem.Parameter{
					Type: gollem.TypeObject,
					Properties: map[string]*gollem.Parameter{
						"id":   {Type: gollem.TypeString, Required: true},
						"size": {Type: gollem.TypeString, Enum: []string{"S", "M", "L"}},
					},
				},
			},
		},
	}
}

func (nestedTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return nil, nil
}

func decodeJSON(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var out map[string]any
	gt.NoError(t, json.Unmarshal(data, &out)).Required()
	return out
}

func TestToolConversion(t *testing.T) {
	t.Run("tool without parameters", func(t *testing.T) {
		data, err := ollama.ToolJSON(noParamTool{})
		gt.NoError(t, err).Required()
		gt.Equal(t, decodeJSON(t, data), map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "now",
				"description": "Current time",
				"parameters":  map[string]any{"type": "object", "properties": map[string]any{}},
			},
		})
	})

	t.Run("nested object, array and enum", func(t *testing.T) {
		data, err := ollama.ToolJSON(nestedTool{})
		gt.NoError(t, err).Required()
		params := decodeJSON(t, data)["function"].(map[string]any)["parameters"].(map[string]any)
		gt.Equal(t, params["properties"], any(map[string]any{
			"items": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []any{"id"},
					"properties": map[string]any{
						"id":   map[string]any{"type": "string"},
						"size": map[string]any{"type": "string", "enum": []any{"S", "M", "L"}},
					},
				},
			},
		}))
		_, hasRequired := params["required"]
		gt.False(t, hasRequired)
	})
}

func TestResponseFormat(t *testing.T) {
	t.Run("text without schema", func(t *testing.T) {
		format, err := ollama.ResponseFormat(gollem.ContentTypeText, nil, nil)
		gt.NoError(t, err).Required()
		gt.Value(t, format).Nil()
	})

	t.Run("schema with a map", func(t *testing.T) {
		param := &gollem.Parameter{
			Type: gollem.TypeObject,
			Properties: map[string]*gollem.Parameter{
				"labels": {
					Type:                 gollem.TypeObject,
					AdditionalProperties: &gollem.Parameter{Type: gollem.TypeString},
				},
			},
		}
		format, err := ollama.ResponseFormat(gollem.ContentTypeJSON, param, nil)
		gt.NoError(t, err).Required()
		labels := decodeJSON(t, format)["properties"].(map[string]any)["labels"].(map[string]any)
		gt.Equal(t, labels["additionalProperties"], any(map[string]any{"type": "string"}))
	})
}
