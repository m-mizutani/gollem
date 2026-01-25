package gollem

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/m-mizutani/goerr/v2"
)

var (
	ErrUnsupportedType = goerr.New("unsupported type for schema conversion")
	ErrInvalidTag      = goerr.New("invalid struct tag")
	ErrCyclicReference = goerr.New("cyclic reference detected")
)

// tagInfo holds parsed tag information from struct fields
type tagInfo struct {
	name        string
	description string
	enum        []string
	min         *float64
	max         *float64
	minLength   *int
	maxLength   *int
	pattern     string
	minItems    *int
	maxItems    *int
	required    bool
	ignore      bool
}

// ToSchema converts a Go struct to gollem.Parameter using reflection.
// It analyzes struct tags to extract metadata like descriptions, constraints, and validation rules.
//
// Supported struct tags:
//   - json:"field_name" - Field name (standard JSON tag)
//   - description:"text" - Field description
//   - enum:"value1,value2,value3" - Enum values (comma-separated)
//   - min:"0" - Minimum value (for numbers)
//   - max:"100" - Maximum value (for numbers)
//   - minLength:"1" - Minimum string length
//   - maxLength:"255" - Maximum string length
//   - pattern:"^[a-z]+$" - Regex pattern for string validation
//   - minItems:"1" - Minimum array length
//   - maxItems:"10" - Maximum array length
//   - required:"true" - Mark field as required
//
// Example:
//
//	type User struct {
//	    Name  string `json:"name" description:"User's full name" required:"true"`
//	    Age   int    `json:"age" description:"Age in years" min:"0" max:"150"`
//	    Email string `json:"email" pattern:"^[a-z@.]+$" required:"true"`
//	    Role  string `json:"role" enum:"admin,user,guest"`
//	}
//
//	schema, err := gollem.ToSchema(User{})
//	if err != nil {
//	    // handle error
//	}
func ToSchema(v any) (*Parameter, error) {
	t := reflect.TypeOf(v)
	if t == nil {
		return nil, goerr.Wrap(ErrUnsupportedType, "nil value")
	}

	return convertWithPath(t, make(map[reflect.Type]bool), tagInfo{})
}

// MustToSchema is like ToSchema but panics on error.
// Useful for static initializations where errors should be caught at development time.
//
// Example:
//
//	var userSchema = gollem.MustToSchema(User{})
func MustToSchema(v any) *Parameter {
	param, err := ToSchema(v)
	if err != nil {
		panic(goerr.Wrap(err, "MustToSchema failed"))
	}
	return param
}

// convertWithPath handles type conversion with cycle detection
func convertWithPath(t reflect.Type, seen map[reflect.Type]bool, tags tagInfo) (*Parameter, error) {
	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Check for cyclic references
	if t.Kind() == reflect.Struct {
		if seen[t] {
			return nil, goerr.Wrap(ErrCyclicReference, "type appears multiple times in hierarchy", goerr.V("type", t.String()))
		}
		seen[t] = true
		defer delete(seen, t)
	}

	param := &Parameter{}

	// Determine parameter type
	switch t.Kind() {
	case reflect.String:
		param.Type = TypeString
		if tags.pattern != "" {
			param.Pattern = tags.pattern
		}
		if tags.minLength != nil {
			param.MinLength = tags.minLength
		}
		if tags.maxLength != nil {
			param.MaxLength = tags.maxLength
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		param.Type = TypeInteger
		if tags.min != nil {
			param.Minimum = tags.min
		}
		if tags.max != nil {
			param.Maximum = tags.max
		}

	case reflect.Float32, reflect.Float64:
		param.Type = TypeNumber
		if tags.min != nil {
			param.Minimum = tags.min
		}
		if tags.max != nil {
			param.Maximum = tags.max
		}

	case reflect.Bool:
		param.Type = TypeBoolean

	case reflect.Slice, reflect.Array:
		param.Type = TypeArray
		elemParam, err := convertWithPath(t.Elem(), seen, tagInfo{})
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert array element type")
		}
		param.Items = elemParam
		if tags.minItems != nil {
			param.MinItems = tags.minItems
		}
		if tags.maxItems != nil {
			param.MaxItems = tags.maxItems
		}

	case reflect.Struct:
		return convertStruct(t, seen)

	case reflect.Map:
		param.Type = TypeObject
		// Maps are treated as objects with arbitrary keys

	default:
		return nil, goerr.Wrap(ErrUnsupportedType, "cannot convert type", goerr.V("type", t.Kind().String()))
	}

	// Apply common tags
	if tags.description != "" {
		param.Description = tags.description
	}
	if len(tags.enum) > 0 {
		param.Enum = tags.enum
	}

	return param, nil
}

// convertStruct converts a struct type to a Parameter with TypeObject
func convertStruct(t reflect.Type, seen map[reflect.Type]bool) (*Parameter, error) {
	param := &Parameter{
		Type:       TypeObject,
		Properties: make(map[string]*Parameter),
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Parse tags
		tags, err := parseTag(field)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to parse tag", goerr.V("field", field.Name))
		}

		// Skip ignored fields
		if tags.ignore {
			continue
		}

		// Use field name from json tag or struct field name
		fieldName := tags.name
		if fieldName == "" {
			fieldName = field.Name
		}

		// Convert field type
		fieldParam, err := convertWithPath(field.Type, seen, tags)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert field", goerr.V("field", field.Name))
		}

		// Set required flag on the parameter itself
		if tags.required {
			fieldParam.Required = true
		}

		param.Properties[fieldName] = fieldParam
	}

	return param, nil
}

// parseTag extracts metadata from struct field tags
func parseTag(field reflect.StructField) (tagInfo, error) {
	info := tagInfo{}

	// Parse json tag for field name
	if jsonTag := field.Tag.Get("json"); jsonTag != "" {
		parts := strings.Split(jsonTag, ",")
		if parts[0] == "-" {
			info.ignore = true
			return info, nil
		}
		if parts[0] != "" {
			info.name = parts[0]
		}
	}

	// Parse description tag
	if desc := field.Tag.Get("description"); desc != "" {
		info.description = desc
	}

	// Parse enum tag
	if enumTag := field.Tag.Get("enum"); enumTag != "" {
		info.enum = strings.Split(enumTag, ",")
		// Trim spaces from enum values
		for i := range info.enum {
			info.enum[i] = strings.TrimSpace(info.enum[i])
		}
	}

	// Parse min tag
	if minTag := field.Tag.Get("min"); minTag != "" {
		val, err := strconv.ParseFloat(minTag, 64)
		if err != nil {
			return info, goerr.Wrap(ErrInvalidTag, "invalid min value", goerr.V("field", field.Name), goerr.V("value", minTag))
		}
		info.min = &val
	}

	// Parse max tag
	if maxTag := field.Tag.Get("max"); maxTag != "" {
		val, err := strconv.ParseFloat(maxTag, 64)
		if err != nil {
			return info, goerr.Wrap(ErrInvalidTag, "invalid max value", goerr.V("field", field.Name), goerr.V("value", maxTag))
		}
		info.max = &val
	}

	// Parse minLength tag
	if minLenTag := field.Tag.Get("minLength"); minLenTag != "" {
		val, err := strconv.Atoi(minLenTag)
		if err != nil {
			return info, goerr.Wrap(ErrInvalidTag, "invalid minLength value", goerr.V("field", field.Name), goerr.V("value", minLenTag))
		}
		info.minLength = &val
	}

	// Parse maxLength tag
	if maxLenTag := field.Tag.Get("maxLength"); maxLenTag != "" {
		val, err := strconv.Atoi(maxLenTag)
		if err != nil {
			return info, goerr.Wrap(ErrInvalidTag, "invalid maxLength value", goerr.V("field", field.Name), goerr.V("value", maxLenTag))
		}
		info.maxLength = &val
	}

	// Parse pattern tag
	if pattern := field.Tag.Get("pattern"); pattern != "" {
		info.pattern = pattern
	}

	// Parse minItems tag
	if minItemsTag := field.Tag.Get("minItems"); minItemsTag != "" {
		val, err := strconv.Atoi(minItemsTag)
		if err != nil {
			return info, goerr.Wrap(ErrInvalidTag, "invalid minItems value", goerr.V("field", field.Name), goerr.V("value", minItemsTag))
		}
		info.minItems = &val
	}

	// Parse maxItems tag
	if maxItemsTag := field.Tag.Get("maxItems"); maxItemsTag != "" {
		val, err := strconv.Atoi(maxItemsTag)
		if err != nil {
			return info, goerr.Wrap(ErrInvalidTag, "invalid maxItems value", goerr.V("field", field.Name), goerr.V("value", maxItemsTag))
		}
		info.maxItems = &val
	}

	// Parse required tag
	if reqTag := field.Tag.Get("required"); reqTag != "" {
		required, err := strconv.ParseBool(reqTag)
		if err != nil {
			return info, goerr.Wrap(ErrInvalidTag, "invalid required value", goerr.V("field", field.Name), goerr.V("value", reqTag))
		}
		info.required = required
	}

	return info, nil
}
