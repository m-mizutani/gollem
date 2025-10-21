# Structured Output with JSON Schema

gollem provides comprehensive support for structured output using JSON Schema, allowing you to constrain LLM responses to specific formats. This ensures predictable, parseable outputs for data extraction, API responses, and structured data generation.

## Overview

By defining a JSON Schema, you can:
- **Guarantee response format**: LLM outputs always conform to your schema
- **Extract structured data**: Parse unstructured text into structured objects
- **Type safety**: Define expected types, constraints, and validation rules
- **Multi-provider support**: Works consistently across OpenAI, Claude, and Gemini

## Basic Usage

### 1. Define a Response Schema

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
		"email": {
			Type:        gollem.TypeString,
			Description: "Email address",
		},
	},
	Required: []string{"name", "email"},
}
```

### 2. Create Session with Schema

```go
session, err := client.NewSession(ctx,
	gollem.WithSessionContentType(gollem.ContentTypeJSON),
	gollem.WithSessionResponseSchema(schema),
)
if err != nil {
	return err
}
```

### 3. Generate Structured Content

```go
resp, err := session.GenerateContent(ctx,
	gollem.Text("Extract user info: John Doe, 30 years old, john@example.com"))
if err != nil {
	return err
}

// Response is guaranteed to be valid JSON matching the schema
var profile map[string]any
json.Unmarshal([]byte(resp.Texts[0]), &profile)
fmt.Printf("Name: %s, Age: %v, Email: %s\n",
	profile["name"], profile["age"], profile["email"])
```

## Schema Parameter Types

gollem supports the following JSON Schema types:

### Basic Types

```go
// String
&gollem.Parameter{
	Type:        gollem.TypeString,
	Description: "A text value",
	Pattern:     "^[a-z]+$",        // Optional regex pattern
	MinLength:   Ptr(1),             // Optional minimum length
	MaxLength:   Ptr(100),           // Optional maximum length
}

// Integer
&gollem.Parameter{
	Type:        gollem.TypeInteger,
	Description: "An integer value",
	Minimum:     Ptr(0.0),           // Optional minimum value
	Maximum:     Ptr(100.0),         // Optional maximum value
}

// Number (floating point)
&gollem.Parameter{
	Type:        gollem.TypeNumber,
	Description: "A numeric value",
	Minimum:     Ptr(0.0),
	Maximum:     Ptr(1.0),
}

// Boolean
&gollem.Parameter{
	Type:        gollem.TypeBoolean,
	Description: "A true/false value",
}
```

### Complex Types

```go
// Object
&gollem.Parameter{
	Type: gollem.TypeObject,
	Properties: map[string]*gollem.Parameter{
		"field1": {Type: gollem.TypeString},
		"field2": {Type: gollem.TypeInteger},
	},
	Required: []string{"field1"},  // Specify required fields
}

// Array
&gollem.Parameter{
	Type: gollem.TypeArray,
	Items: &gollem.Parameter{
		Type: gollem.TypeString,
	},
	MinItems: Ptr(1),   // Optional minimum array length
	MaxItems: Ptr(10),  // Optional maximum array length
}

// Enum (restricted values)
&gollem.Parameter{
	Type: gollem.TypeString,
	Enum: []any{"active", "inactive", "pending"},
	Description: "User status",
}
```

## Advanced Examples

### Nested Objects

```go
schema := &gollem.ResponseSchema{
	Name: "Organization",
	Schema: &gollem.Parameter{
		Type: gollem.TypeObject,
		Properties: map[string]*gollem.Parameter{
			"name": {
				Type: gollem.TypeString,
			},
			"address": {
				Type: gollem.TypeObject,
				Properties: map[string]*gollem.Parameter{
					"street": {Type: gollem.TypeString},
					"city":   {Type: gollem.TypeString},
					"country": {Type: gollem.TypeString},
					"zipcode": {
						Type:    gollem.TypeString,
						Pattern: "^[0-9]{5}$",
					},
				},
				Required: []string{"city", "country"},
			},
		},
		Required: []string{"name", "address"},
	},
}
```

### Arrays of Objects

```go
schema := &gollem.ResponseSchema{
	Name: "EmployeeList",
	Schema: &gollem.Parameter{
		Type: gollem.TypeObject,
		Properties: map[string]*gollem.Parameter{
			"employees": {
				Type: gollem.TypeArray,
				Items: &gollem.Parameter{
					Type: gollem.TypeObject,
					Properties: map[string]*gollem.Parameter{
						"name": {
							Type: gollem.TypeString,
						},
						"position": {
							Type: gollem.TypeString,
							Enum: []any{"engineer", "manager", "designer"},
						},
						"salary": {
							Type:    gollem.TypeInteger,
							Minimum: Ptr(0.0),
						},
					},
					Required: []string{"name", "position"},
				},
				MinItems: Ptr(1),
			},
		},
	},
}
```

### Complex Data Extraction

```go
schema := &gollem.ResponseSchema{
	Name:        "SecurityAlert",
	Description: "Structured security alert analysis",
	Schema: &gollem.Parameter{
		Type: gollem.TypeObject,
		Properties: map[string]*gollem.Parameter{
			"severity": {
				Type: gollem.TypeString,
				Enum: []any{"low", "medium", "high", "critical"},
			},
			"threat_type": {
				Type: gollem.TypeString,
			},
			"affected_systems": {
				Type: gollem.TypeArray,
				Items: &gollem.Parameter{
					Type: gollem.TypeString,
				},
			},
			"indicators": {
				Type: gollem.TypeObject,
				Properties: map[string]*gollem.Parameter{
					"ip_addresses": {
						Type: gollem.TypeArray,
						Items: &gollem.Parameter{Type: gollem.TypeString},
					},
					"domains": {
						Type: gollem.TypeArray,
						Items: &gollem.Parameter{Type: gollem.TypeString},
					},
					"file_hashes": {
						Type: gollem.TypeArray,
						Items: &gollem.Parameter{Type: gollem.TypeString},
					},
				},
			},
			"recommended_actions": {
				Type: gollem.TypeArray,
				Items: &gollem.Parameter{
					Type: gollem.TypeString,
				},
				MinItems: Ptr(1),
			},
		},
		Required: []string{"severity", "threat_type", "recommended_actions"},
	},
}
```

## Creating Schemas from Go Structs

### ToSchema

Instead of manually constructing `gollem.Parameter` objects, you can automatically generate schemas from Go struct types using struct field tags:

```go
type UserProfile struct {
	Name     string  `json:"name" description:"User's full name" required:"true"`
	Email    string  `json:"email" description:"Email address" pattern:"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$" required:"true"`
	Age      int     `json:"age" description:"Age in years" min:"0" max:"150"`
	Role     string  `json:"role" description:"User role" enum:"admin,user,guest"`
	Username string  `json:"username" description:"Unique username" minLength:"3" maxLength:"20" pattern:"^[a-z0-9]+$" required:"true"`
	Bio      string  `json:"bio" description:"User biography" maxLength:"500"`
	Address  Address `json:"address" description:"User's address"`
	Tags     []string `json:"tags" description:"User tags" minItems:"1" maxItems:"10"`
}

type Address struct {
	Street  string `json:"street" required:"true"`
	City    string `json:"city" required:"true"`
	Country string `json:"country" required:"true"`
	ZipCode string `json:"zip_code" pattern:"^[0-9]{5}$"`
}

// Convert struct to Parameter
param, err := gollem.ToSchema(UserProfile{})
if err != nil {
	return err
}

// Set Title and Description
param.Title = "UserProfile"
param.Description = "Structured user profile information"

// Use with LLM
session, err := client.NewSession(ctx,
	gollem.WithSessionContentType(gollem.ContentTypeJSON),
	gollem.WithSessionResponseSchema(param),
)
```

### Supported Struct Tags

All tags are optional except `json` for field naming:

- **`json:"field_name"`** - Field name in JSON (standard Go tag)
  - Use `json:"-"` to ignore fields
- **`description:"text"`** - Field description
- **`enum:"value1,value2,value3"`** - Enum values (comma-separated)
- **`min:"0"`** - Minimum value (for numbers)
- **`max:"100"`** - Maximum value (for numbers)
- **`minLength:"1"`** - Minimum string length
- **`maxLength:"255"`** - Maximum string length
- **`pattern:"^[a-z]+$"`** - Regex pattern for string validation
- **`minItems:"1"`** - Minimum array length
- **`maxItems:"10"`** - Maximum array length
- **`required:"true"`** - Mark field as required

### Supported Types

- **Basic types**: `string`, `int`, `int8`, `int16`, `int32`, `int64`, `uint`, `uint8`, `uint16`, `uint32`, `uint64`, `float32`, `float64`, `bool`
- **Complex types**: `[]T` (arrays/slices), nested structs, `map[string]T`
- **Pointer types**: Automatically unwrapped to base type

### MustToSchema

For static initializations where errors should be caught at development time:

```go
var userSchema = gollem.MustToSchema(UserProfile{})
// Panics if conversion fails
```

### Complete Example

See [examples/json_schema](../examples/json_schema) for a complete working example.

## Helper Functions

### Pointer Utility

Since many schema fields accept pointers, use this helper:

```go
func Ptr[T any](v T) *T {
	return &v
}

// Usage
schema := &gollem.Parameter{
	Type:    gollem.TypeInteger,
	Minimum: Ptr(0.0),
	Maximum: Ptr(100.0),
}
```

## Provider-Specific Behavior

### OpenAI

OpenAI uses Structured Outputs with JSON Schema:
- Supports all JSON Schema features
- `strict: true` is automatically enabled for schema validation
- Compatible with GPT-4o and later models

### Claude

Claude uses the JSON Schema in system instructions:
- Schema is converted to clear JSON format description
- Works with all Claude 3+ models
- Automatically enforces JSON output

### Gemini

Gemini uses `response_schema` parameter:
- Native JSON Schema support via Vertex AI
- Works with Gemini 1.5 Pro and later
- Automatically validates response format

## Best Practices

### 1. Provide Clear Descriptions

```go
&gollem.Parameter{
	Type:        gollem.TypeString,
	Description: "User's full legal name as it appears on government ID",
	// Better than: Description: "name"
}
```

### 2. Use Appropriate Constraints

```go
// Email validation
&gollem.Parameter{
	Type:    gollem.TypeString,
	Pattern: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$",
}

// Age constraint
&gollem.Parameter{
	Type:    gollem.TypeInteger,
	Minimum: Ptr(0.0),
	Maximum: Ptr(150.0),
}
```

### 3. Mark Required Fields

```go
&gollem.Parameter{
	Type: gollem.TypeObject,
	Properties: map[string]*gollem.Parameter{
		"required_field": {Type: gollem.TypeString},
		"optional_field": {Type: gollem.TypeString},
	},
	Required: []string{"required_field"},
}
```

### 4. Use Enums for Fixed Values

```go
&gollem.Parameter{
	Type: gollem.TypeString,
	Enum: []any{"pending", "approved", "rejected"},
	Description: "Application status - must be one of the allowed values",
}
```

## Common Use Cases

### Data Extraction

Extract structured information from unstructured text:

```go
schema := &gollem.ResponseSchema{
	Name: "InvoiceData",
	Schema: &gollem.Parameter{
		Type: gollem.TypeObject,
		Properties: map[string]*gollem.Parameter{
			"invoice_number": {Type: gollem.TypeString},
			"date":           {Type: gollem.TypeString},
			"total_amount":   {Type: gollem.TypeNumber},
			"items": {
				Type: gollem.TypeArray,
				Items: &gollem.Parameter{
					Type: gollem.TypeObject,
					Properties: map[string]*gollem.Parameter{
						"description": {Type: gollem.TypeString},
						"quantity":    {Type: gollem.TypeInteger},
						"price":       {Type: gollem.TypeNumber},
					},
				},
			},
		},
	},
}
```

### Classification

Classify content into predefined categories:

```go
schema := &gollem.ResponseSchema{
	Name: "ContentClassification",
	Schema: &gollem.Parameter{
		Type: gollem.TypeObject,
		Properties: map[string]*gollem.Parameter{
			"category": {
				Type: gollem.TypeString,
				Enum: []any{"technical", "business", "personal", "other"},
			},
			"subcategory": {Type: gollem.TypeString},
			"confidence": {
				Type:    gollem.TypeNumber,
				Minimum: Ptr(0.0),
				Maximum: Ptr(1.0),
			},
		},
		Required: []string{"category", "confidence"},
	},
}
```

### Form Generation

Generate structured forms or questionnaires:

```go
schema := &gollem.ResponseSchema{
	Name: "SurveyQuestions",
	Schema: &gollem.Parameter{
		Type: gollem.TypeObject,
		Properties: map[string]*gollem.Parameter{
			"questions": {
				Type: gollem.TypeArray,
				Items: &gollem.Parameter{
					Type: gollem.TypeObject,
					Properties: map[string]*gollem.Parameter{
						"question": {Type: gollem.TypeString},
						"type": {
							Type: gollem.TypeString,
							Enum: []any{"multiple_choice", "text", "rating"},
						},
						"options": {
							Type:  gollem.TypeArray,
							Items: &gollem.Parameter{Type: gollem.TypeString},
						},
					},
					Required: []string{"question", "type"},
				},
				MinItems: Ptr(3),
				MaxItems: Ptr(10),
			},
		},
	},
}
```

## Error Handling

When schema validation fails (rare with proper setup):

```go
resp, err := session.GenerateContent(ctx, gollem.Text("..."))
if err != nil {
	return fmt.Errorf("failed to generate content: %w", err)
}

var result map[string]any
if err := json.Unmarshal([]byte(resp.Texts[0]), &result); err != nil {
	return fmt.Errorf("failed to parse JSON response: %w", err)
}
```

## Complete Example

See [examples/json_schema](../examples/json_schema) for a complete working example demonstrating:
- Schema definition for user profiles
- Extraction of structured data from natural language
- Usage with OpenAI, Claude, and Gemini
- Pretty-printing JSON output

## Related Documentation

- [Getting Started Guide](getting-started.md)
- [Tool Development](tools.md)
- [LLM Provider Configuration](llm.md)
