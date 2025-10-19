package gollem

import "github.com/m-mizutani/goerr/v2"

// ResponseSchema defines the schema for JSON response output.
// It reuses the Parameter structure to maintain consistency with tool parameter definitions.
type ResponseSchema struct {
	// Schema is the root schema definition for the response.
	// It should typically be of TypeObject with Properties defined.
	Schema *Parameter

	// Name is an optional name for the schema (used by some providers like OpenAI).
	Name string

	// Description provides context about the expected response structure.
	Description string
}

// Validate validates the response schema.
func (rs *ResponseSchema) Validate() error {
	if rs == nil {
		return nil
	}

	if rs.Schema == nil {
		return goerr.Wrap(ErrInvalidParameter, "schema is required")
	}

	// Validate using existing Parameter.Validate()
	if err := rs.Schema.Validate(); err != nil {
		return goerr.Wrap(err, "invalid schema")
	}

	// Schema should be TypeObject for root
	if rs.Schema.Type != TypeObject {
		return goerr.Wrap(ErrInvalidParameter,
			"response schema root must be an object type")
	}

	return nil
}
