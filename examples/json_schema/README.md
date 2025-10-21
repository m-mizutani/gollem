# JSON Schema Example

This example demonstrates how to use JSON Schema support in gollem to get structured JSON output from LLM providers (OpenAI, Claude, and Gemini).

## Features

- Define a structured schema for LLM responses
- Extract structured data from natural language
- Support for nested objects and arrays
- Field validation with constraints
- Works across multiple LLM providers

## Prerequisites

Set the appropriate environment variables for the providers you want to use:

```bash
# For OpenAI
export OPENAI_API_KEY="your-openai-api-key"

# For Claude
export ANTHROPIC_API_KEY="your-anthropic-api-key"

# For Gemini
export GEMINI_PROJECT_ID="your-gcp-project-id"
export GEMINI_LOCATION="us-central1"  # or your preferred location
```

## Running the Example

```bash
go run main.go
```

The example will try to run all three providers. If an API key is not set, that provider will be skipped.

## Code Overview

### Schema Definition

The example defines a `UserProfile` schema with the following fields:

- `name` (string, required): Full name of the user
- `age` (integer): Age in years (0-150)
- `email` (string, required): Email address
- `interests` (array of strings): List of hobbies or interests
- `location` (object): User's location with `city` and `country` fields

```go
schema := &gollem.Parameter{
    Title:       "UserProfile",
    Description: "Structured user profile information",
    Type:        gollem.TypeObject,
    Properties: map[string]*gollem.Parameter{
        "name": {
            Type:        gollem.TypeString,
            Description: "Full name of the user",
        },
        "age": {
            Type:        gollem.TypeInteger,
            Description: "Age in years",
            Minimum:     Ptr(0.0),
            Maximum:     Ptr(150.0),
        },
        // ... more fields
    },
    Required: []string{"name", "email"},
}
```

### Session Creation

Create a session with JSON content type and response schema:

```go
session, err := client.NewSession(ctx,
    gollem.WithSessionContentType(gollem.ContentTypeJSON),
    gollem.WithSessionResponseSchema(schema),
)
```

### Generate Structured Output

Send a natural language prompt and receive structured JSON:

```go
resp, err := session.GenerateContent(ctx,
    gollem.Text("Extract user information: Sarah Johnson is 28 years old, email: sarah.j@example.com, lives in Seattle, USA, and enjoys hiking, photography, and cooking."))

// Response will be a valid JSON matching the schema
```

## Provider-Specific Notes

### OpenAI

- Uses Structured Outputs with `response_format.json_schema`
- Uses strict mode (internal default: `strict=false` for flexibility)
- Requires `gpt-4o-2024-08-06` or later models
- Constrained decoding ensures high schema adherence

### Claude

- Uses system prompt + prefill technique
- Schema is embedded in the system prompt
- Prefills response with `{` to enforce JSON format
- May have slightly lower accuracy than native schema support

### Gemini

- Uses `ResponseSchema` API parameter
- Native schema support via Vertex AI
- Works with Gemini 1.5 Pro and Flash models
- Complex schemas may trigger `InvalidArgument: 400` errors

## Expected Output

```json
{
  "name": "Sarah Johnson",
  "age": 28,
  "email": "sarah.j@example.com",
  "interests": [
    "hiking",
    "photography",
    "cooking"
  ],
  "location": {
    "city": "Seattle",
    "country": "USA"
  }
}
```

## Schema Features

The example demonstrates several schema features:

1. **Required fields**: `name` and `email` must be present
2. **Type validation**: Each field has a specific type
3. **Nested objects**: `location` is an object with sub-fields
4. **Arrays**: `interests` is an array of strings
5. **Constraints**: `age` has minimum and maximum values
6. **Descriptions**: Each field has a description for the LLM

## Error Handling

The example includes proper error handling for:
- Missing API keys
- Session creation failures
- Content generation errors
- JSON parsing errors

## Additional Resources

- [gollem Documentation](../../README.md)
- [OpenAI Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs)
- [Claude Prefill Technique](https://docs.anthropic.com/claude/docs/prefill-claudes-response)
- [Gemini Response Schema](https://cloud.google.com/vertex-ai/generative-ai/docs/multimodal/control-generated-output)
