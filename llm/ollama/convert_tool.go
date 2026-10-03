package ollama

import (
	"encoding/json"

	"github.com/gollem-dev/gollem"
	"github.com/gollem-dev/gollem/internal/schema"
	"github.com/m-mizutani/goerr/v2"
)

// formatJSON is the "format" value that asks for JSON output without a schema.
var formatJSON = json.RawMessage(`"json"`)

// convertTool converts a gollem.Tool to the Ollama tool definition.
//
// The server reads each parameter into api.ToolProperty, which keeps only
// type, description, enum, items, properties, required and anyOf. Constraints
// such as minimum or pattern are sent but not used by the server.
func convertTool(t gollem.Tool) tool {
	spec := t.Spec()

	properties := make(map[string]any, len(spec.Parameters))
	for name, param := range spec.Parameters {
		properties[name] = schema.ConvertParameterToJSONSchema(param)
	}

	// The server declares "properties" without omitempty, so an empty object
	// is sent for a tool without parameters.
	parameters := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if required := schema.CollectRequiredFields(spec.Parameters); len(required) > 0 {
		parameters["required"] = required
	}

	return tool{
		Type: "function",
		Function: toolFunction{
			Name:        spec.Name,
			Description: spec.Description,
			Parameters:  parameters,
		},
	}
}

// responseFormat returns the "format" value for a request. A per-call schema
// takes precedence over the session schema. Without a schema it asks for JSON
// when the session content type is JSON, and returns nil otherwise.
func responseFormat(contentType gollem.ContentType, sessionSchema, callSchema *gollem.Parameter) (json.RawMessage, error) {
	param := callSchema
	if param == nil {
		param = sessionSchema
	}

	if param == nil {
		if contentType == gollem.ContentTypeJSON {
			return formatJSON, nil
		}
		return nil, nil
	}

	if err := param.Validate(); err != nil {
		return nil, goerr.Wrap(err, "invalid response schema")
	}
	data, err := json.Marshal(schema.ConvertParameterToJSONSchema(param))
	if err != nil {
		return nil, goerr.Wrap(err, "failed to marshal response schema")
	}
	return data, nil
}
