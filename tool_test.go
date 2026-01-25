package gollem_test

import (
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
)

func TestParameterValidation(t *testing.T) {
	t.Run("number constraints", func(t *testing.T) {
		t.Run("valid minimum and maximum", func(t *testing.T) {
			p := &gollem.Parameter{
				Type:    gollem.TypeNumber,
				Minimum: ptr(1.0),
				Maximum: ptr(10.0),
			}
			gt.NoError(t, p.Validate())
		})

		t.Run("invalid minimum and maximum", func(t *testing.T) {
			p := &gollem.Parameter{
				Type:    gollem.TypeNumber,
				Minimum: ptr(10.0),
				Maximum: ptr(1.0),
			}
			gt.Error(t, p.Validate())
		})
	})

	t.Run("string constraints", func(t *testing.T) {
		t.Run("valid minLength and maxLength", func(t *testing.T) {
			p := &gollem.Parameter{
				Type:      gollem.TypeString,
				MinLength: ptr(1),
				MaxLength: ptr(10),
			}
			gt.NoError(t, p.Validate())
		})

		t.Run("invalid minLength and maxLength", func(t *testing.T) {
			p := &gollem.Parameter{
				Type:      gollem.TypeString,
				MinLength: ptr(10),
				MaxLength: ptr(1),
			}
			gt.Error(t, p.Validate())
		})

		t.Run("valid pattern", func(t *testing.T) {
			p := &gollem.Parameter{
				Type:    gollem.TypeString,
				Pattern: "^[a-z]+$",
			}
			gt.NoError(t, p.Validate())
		})

		t.Run("invalid pattern", func(t *testing.T) {
			p := &gollem.Parameter{
				Type:    gollem.TypeString,
				Pattern: "[invalid",
			}
			gt.Error(t, p.Validate())
		})
	})

	t.Run("array constraints", func(t *testing.T) {
		t.Run("valid minItems and maxItems", func(t *testing.T) {
			p := &gollem.Parameter{
				Type:     gollem.TypeArray,
				Items:    &gollem.Parameter{Type: gollem.TypeString},
				MinItems: ptr(1),
				MaxItems: ptr(10),
			}
			gt.NoError(t, p.Validate())
		})

		t.Run("invalid minItems and maxItems", func(t *testing.T) {
			p := &gollem.Parameter{
				Type:     gollem.TypeArray,
				Items:    &gollem.Parameter{Type: gollem.TypeString},
				MinItems: ptr(10),
				MaxItems: ptr(1),
			}
			gt.Error(t, p.Validate())
		})
	})

	t.Run("object constraints", func(t *testing.T) {
		t.Run("valid properties", func(t *testing.T) {
			p := &gollem.Parameter{
				Type: gollem.TypeObject,
				Properties: map[string]*gollem.Parameter{
					"name": {
						Type:        gollem.TypeString,
						Description: "User name",
					},
					"age": {
						Type:        gollem.TypeNumber,
						Description: "User age",
					},
				},
			}
			gt.NoError(t, p.Validate())
		})

		t.Run("duplicate property names", func(t *testing.T) {
			p := &gollem.Parameter{
				Type:       gollem.TypeObject,
				Properties: make(map[string]*gollem.Parameter),
			}
			p.Properties["name"] = &gollem.Parameter{
				Type:        gollem.TypeString,
				Description: "User name",
			}
			p.Properties["name"] = &gollem.Parameter{
				Type:        gollem.TypeString,
				Description: "Duplicate name",
			}
			gt.NoError(t, p.Validate())
		})

		t.Run("invalid property type", func(t *testing.T) {
			p := &gollem.Parameter{
				Type: gollem.TypeObject,
				Properties: map[string]*gollem.Parameter{
					"name": {
						Type:        "invalid",
						Description: "User name",
					},
				},
			}
			gt.Error(t, p.Validate())
		})
	})
}

func ptr[T any](v T) *T {
	return &v
}

func TestToolSpecValidation(t *testing.T) {
	t.Run("valid tool spec", func(t *testing.T) {
		spec := gollem.ToolSpec{
			Name:        "test",
			Description: "test description",
			Parameters: map[string]*gollem.Parameter{
				"param1": {
					Type:        gollem.TypeString,
					Description: "test parameter",
					Required:    true,
				},
			},
		}
		gt.NoError(t, spec.Validate())
	})

	t.Run("empty name", func(t *testing.T) {
		spec := gollem.ToolSpec{
			Description: "test description",
			Parameters: map[string]*gollem.Parameter{
				"param1": {
					Type:        gollem.TypeString,
					Description: "test parameter",
				},
			},
		}
		gt.Error(t, spec.Validate())
	})

	t.Run("invalid parameter type", func(t *testing.T) {
		spec := gollem.ToolSpec{
			Name:        "test",
			Description: "test description",
			Parameters: map[string]*gollem.Parameter{
				"param1": {
					Type:        "invalid",
					Description: "test parameter",
				},
			},
		}
		gt.Error(t, spec.Validate())
	})

	t.Run("invalid parameter", func(t *testing.T) {
		spec := gollem.ToolSpec{
			Name:        "test",
			Description: "test description",
			Parameters: map[string]*gollem.Parameter{
				"param1": {
					Type:    gollem.TypeNumber,
					Minimum: ptr(10.0),
					Maximum: ptr(1.0),
				},
			},
		}
		gt.Error(t, spec.Validate())
	})

	t.Run("object parameter without properties", func(t *testing.T) {
		spec := gollem.ToolSpec{
			Name:        "test",
			Description: "test description",
			Parameters: map[string]*gollem.Parameter{
				"param1": {
					Type:        gollem.TypeObject,
					Description: "test parameter",
				},
			},
		}
		gt.Error(t, spec.Validate())
	})

	t.Run("array parameter without items", func(t *testing.T) {
		spec := gollem.ToolSpec{
			Name:        "test",
			Description: "test description",
			Parameters: map[string]*gollem.Parameter{
				"param1": {
					Type:        gollem.TypeArray,
					Description: "test parameter",
				},
			},
		}
		gt.Error(t, spec.Validate())
	})
}

func TestValidateValue(t *testing.T) {
	t.Run("required parameter", func(t *testing.T) {
		t.Run("nil value returns error", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeString, Required: true}
			gt.Error(t, p.ValidateValue("test", nil))
		})

		t.Run("valid value passes", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeString, Required: true}
			gt.NoError(t, p.ValidateValue("test", "hello"))
		})
	})

	t.Run("optional parameter", func(t *testing.T) {
		t.Run("nil value is valid", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeString, Required: false}
			gt.NoError(t, p.ValidateValue("test", nil))
		})
	})

	t.Run("string type", func(t *testing.T) {
		t.Run("valid string", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeString}
			gt.NoError(t, p.ValidateValue("test", "hello"))
		})

		t.Run("non-string returns error", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeString}
			gt.Error(t, p.ValidateValue("test", 123))
		})

		t.Run("enum validation passes", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeString, Enum: []string{"a", "b", "c"}}
			gt.NoError(t, p.ValidateValue("test", "b"))
		})

		t.Run("enum validation fails", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeString, Enum: []string{"a", "b", "c"}}
			gt.Error(t, p.ValidateValue("test", "d"))
		})

		t.Run("minLength validation", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeString, MinLength: ptr(3)}
			gt.Error(t, p.ValidateValue("test", "ab"))
			gt.NoError(t, p.ValidateValue("test", "abc"))
		})

		t.Run("maxLength validation", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeString, MaxLength: ptr(3)}
			gt.NoError(t, p.ValidateValue("test", "abc"))
			gt.Error(t, p.ValidateValue("test", "abcd"))
		})

		t.Run("pattern validation", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeString, Pattern: "^[a-z]+$"}
			gt.NoError(t, p.ValidateValue("test", "abc"))
			gt.Error(t, p.ValidateValue("test", "ABC"))
		})
	})

	t.Run("number type", func(t *testing.T) {
		t.Run("valid float64", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeNumber}
			gt.NoError(t, p.ValidateValue("test", 3.14))
		})

		t.Run("valid int", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeNumber}
			gt.NoError(t, p.ValidateValue("test", 42))
		})

		t.Run("non-number returns error", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeNumber}
			gt.Error(t, p.ValidateValue("test", "not a number"))
		})

		t.Run("minimum validation", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeNumber, Minimum: ptr(5.0)}
			gt.Error(t, p.ValidateValue("test", 4.0))
			gt.NoError(t, p.ValidateValue("test", 5.0))
		})

		t.Run("maximum validation", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeNumber, Maximum: ptr(10.0)}
			gt.NoError(t, p.ValidateValue("test", 10.0))
			gt.Error(t, p.ValidateValue("test", 11.0))
		})
	})

	t.Run("integer type", func(t *testing.T) {
		t.Run("valid int", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeInteger}
			gt.NoError(t, p.ValidateValue("test", 42))
		})

		t.Run("float64 that is integer is valid", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeInteger}
			gt.NoError(t, p.ValidateValue("test", float64(42)))
		})

		t.Run("non-integer float64 returns error", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeInteger}
			gt.Error(t, p.ValidateValue("test", 3.14))
		})
	})

	t.Run("boolean type", func(t *testing.T) {
		t.Run("valid bool", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeBoolean}
			gt.NoError(t, p.ValidateValue("test", true))
			gt.NoError(t, p.ValidateValue("test", false))
		})

		t.Run("non-bool returns error", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeBoolean}
			gt.Error(t, p.ValidateValue("test", "true"))
		})
	})

	t.Run("array type", func(t *testing.T) {
		t.Run("valid array", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeArray, Items: &gollem.Parameter{Type: gollem.TypeString}}
			gt.NoError(t, p.ValidateValue("test", []any{"a", "b", "c"}))
		})

		t.Run("non-array returns error", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeArray, Items: &gollem.Parameter{Type: gollem.TypeString}}
			gt.Error(t, p.ValidateValue("test", "not an array"))
		})

		t.Run("minItems validation", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeArray, Items: &gollem.Parameter{Type: gollem.TypeString}, MinItems: ptr(2)}
			gt.Error(t, p.ValidateValue("test", []any{"a"}))
			gt.NoError(t, p.ValidateValue("test", []any{"a", "b"}))
		})

		t.Run("maxItems validation", func(t *testing.T) {
			p := &gollem.Parameter{Type: gollem.TypeArray, Items: &gollem.Parameter{Type: gollem.TypeString}, MaxItems: ptr(2)}
			gt.NoError(t, p.ValidateValue("test", []any{"a", "b"}))
			gt.Error(t, p.ValidateValue("test", []any{"a", "b", "c"}))
		})
	})

	t.Run("object type", func(t *testing.T) {
		t.Run("valid object", func(t *testing.T) {
			p := &gollem.Parameter{
				Type: gollem.TypeObject,
				Properties: map[string]*gollem.Parameter{
					"name": {Type: gollem.TypeString, Required: true},
					"age":  {Type: gollem.TypeNumber},
				},
			}
			gt.NoError(t, p.ValidateValue("test", map[string]any{"name": "Alice", "age": 30.0}))
		})

		t.Run("non-object returns error", func(t *testing.T) {
			p := &gollem.Parameter{
				Type: gollem.TypeObject,
				Properties: map[string]*gollem.Parameter{
					"name": {Type: gollem.TypeString},
				},
			}
			gt.Error(t, p.ValidateValue("test", "not an object"))
		})

		t.Run("missing required property returns error", func(t *testing.T) {
			p := &gollem.Parameter{
				Type: gollem.TypeObject,
				Properties: map[string]*gollem.Parameter{
					"name": {Type: gollem.TypeString, Required: true},
				},
			}
			gt.Error(t, p.ValidateValue("test", map[string]any{}))
		})

		t.Run("invalid property type returns error", func(t *testing.T) {
			p := &gollem.Parameter{
				Type: gollem.TypeObject,
				Properties: map[string]*gollem.Parameter{
					"age": {Type: gollem.TypeNumber},
				},
			}
			gt.Error(t, p.ValidateValue("test", map[string]any{"age": "not a number"}))
		})
	})
}
