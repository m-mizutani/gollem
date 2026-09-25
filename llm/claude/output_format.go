package claude

import (
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/gollem-dev/gollem"
	gollemschema "github.com/gollem-dev/gollem/internal/schema"
	"github.com/m-mizutani/goerr/v2"
)

// outputFormat converts a response schema into the output_config.format value
// of a Messages request.
//
// With output_config.format the API decodes the response under the schema, so
// the text is valid JSON matching it. The documented exceptions are a refusal
// (stop_reason "refusal"), a response cut off by max_tokens, and a string enum
// value returned with different capitalization. None is repaired here, because
// rewriting such text would hand the caller a value the model did not produce.
func outputFormat(param *gollem.Parameter) (anthropic.JSONOutputFormatParam, error) {
	if err := param.Validate(); err != nil {
		return anthropic.JSONOutputFormatParam{}, goerr.Wrap(err, "invalid response schema")
	}
	return anthropic.JSONOutputFormatParam{Schema: outputSchema(param)}, nil
}

// outputSchema converts a parameter into the JSON Schema subset that
// structured outputs accepts.
//
// Structured outputs rejects numeric bounds, string length and pattern
// constraints, maxItems and a minItems above 1, and requires
// additionalProperties to be false on every object. The rejected constraints
// are removed from the schema and restated in the description, so the model is
// still told about them; the API does not enforce them and this package does
// not validate the response against them.
func outputSchema(param *gollem.Parameter) map[string]any {
	schema := map[string]any{
		"type": string(param.Type),
	}

	if param.Type == gollem.TypeObject && param.Properties != nil {
		props := make(map[string]any, len(param.Properties))
		for name, prop := range param.Properties {
			props[name] = outputSchema(prop)
		}
		schema["properties"] = props
		schema["additionalProperties"] = false
		if required := gollemschema.CollectRequiredFields(param.Properties); len(required) > 0 {
			schema["required"] = required
		}
	}

	if param.Type == gollem.TypeArray && param.Items != nil {
		schema["items"] = outputSchema(param.Items)
	}

	if param.Enum != nil {
		schema["enum"] = param.Enum
	}

	if param.Default != nil {
		schema["default"] = param.Default
	}

	// The order is fixed so that identical schemas produce identical request
	// bytes; a changed schema invalidates the compiled grammar and prompt cache.
	var constraints []string
	if param.Minimum != nil {
		constraints = append(constraints, fmt.Sprintf("minimum: %v", *param.Minimum))
	}
	if param.Maximum != nil {
		constraints = append(constraints, fmt.Sprintf("maximum: %v", *param.Maximum))
	}
	if param.MinLength != nil {
		constraints = append(constraints, fmt.Sprintf("minLength: %d", *param.MinLength))
	}
	if param.MaxLength != nil {
		constraints = append(constraints, fmt.Sprintf("maxLength: %d", *param.MaxLength))
	}
	if param.Pattern != "" {
		constraints = append(constraints, fmt.Sprintf("pattern: %s", param.Pattern))
	}
	if param.MinItems != nil {
		if *param.MinItems <= 1 {
			schema["minItems"] = *param.MinItems
		} else {
			constraints = append(constraints, fmt.Sprintf("minItems: %d", *param.MinItems))
		}
	}
	if param.MaxItems != nil {
		constraints = append(constraints, fmt.Sprintf("maxItems: %d", *param.MaxItems))
	}

	description := param.Description
	if len(constraints) > 0 {
		note := "{" + strings.Join(constraints, ", ") + "}"
		if description != "" {
			description += "\n\n" + note
		} else {
			description = note
		}
	}
	if description != "" {
		schema["description"] = description
	}

	return schema
}
