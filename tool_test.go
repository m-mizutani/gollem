package gollem

import (
	"testing"

	"github.com/m-mizutani/gt"
)

func TestParameterValidation(t *testing.T) {
	t.Run("number constraints", func(t *testing.T) {
		t.Run("valid minimum and maximum", func(t *testing.T) {
			p := &Parameter{
				Type:    TypeNumber,
				Minimum: ptr(1.0),
				Maximum: ptr(10.0),
			}
			gt.NoError(t, p.Validate())
		})

		t.Run("invalid minimum and maximum", func(t *testing.T) {
			p := &Parameter{
				Type:    TypeNumber,
				Minimum: ptr(10.0),
				Maximum: ptr(1.0),
			}
			gt.Error(t, p.Validate())
		})
	})

	t.Run("string constraints", func(t *testing.T) {
		t.Run("valid minLength and maxLength", func(t *testing.T) {
			p := &Parameter{
				Type:      TypeString,
				MinLength: ptr(1),
				MaxLength: ptr(10),
			}
			gt.NoError(t, p.Validate())
		})

		t.Run("invalid minLength and maxLength", func(t *testing.T) {
			p := &Parameter{
				Type:      TypeString,
				MinLength: ptr(10),
				MaxLength: ptr(1),
			}
			gt.Error(t, p.Validate())
		})

		t.Run("valid pattern", func(t *testing.T) {
			p := &Parameter{
				Type:    TypeString,
				Pattern: "^[a-z]+$",
			}
			gt.NoError(t, p.Validate())
		})

		t.Run("invalid pattern", func(t *testing.T) {
			p := &Parameter{
				Type:    TypeString,
				Pattern: "[invalid",
			}
			gt.Error(t, p.Validate())
		})
	})

	t.Run("array constraints", func(t *testing.T) {
		t.Run("valid minItems and maxItems", func(t *testing.T) {
			p := &Parameter{
				Type:     TypeArray,
				Items:    &Parameter{Type: TypeString},
				MinItems: ptr(1),
				MaxItems: ptr(10),
			}
			gt.NoError(t, p.Validate())
		})

		t.Run("invalid minItems and maxItems", func(t *testing.T) {
			p := &Parameter{
				Type:     TypeArray,
				Items:    &Parameter{Type: TypeString},
				MinItems: ptr(10),
				MaxItems: ptr(1),
			}
			gt.Error(t, p.Validate())
		})
	})

	t.Run("object constraints", func(t *testing.T) {
		t.Run("valid properties", func(t *testing.T) {
			p := &Parameter{
				Type: TypeObject,
				Properties: map[string]*Parameter{
					"name": {
						Type:        TypeString,
						Description: "User name",
					},
					"age": {
						Type:        TypeNumber,
						Description: "User age",
					},
				},
			}
			gt.NoError(t, p.Validate())
		})

		t.Run("duplicate property names", func(t *testing.T) {
			p := &Parameter{
				Type:       TypeObject,
				Properties: make(map[string]*Parameter),
			}
			p.Properties["name"] = &Parameter{
				Type:        TypeString,
				Description: "User name",
			}
			p.Properties["name"] = &Parameter{
				Type:        TypeString,
				Description: "Duplicate name",
			}
			gt.NoError(t, p.Validate())
		})

		t.Run("invalid property type", func(t *testing.T) {
			p := &Parameter{
				Type: TypeObject,
				Properties: map[string]*Parameter{
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
		spec := ToolSpec{
			Name:        "test",
			Description: "test description",
			Parameters: map[string]*Parameter{
				"param1": {
					Type:        TypeString,
					Description: "test parameter",
				},
			},
			Required: []string{"param1"},
		}
		gt.NoError(t, spec.Validate())
	})

	t.Run("empty name", func(t *testing.T) {
		spec := ToolSpec{
			Description: "test description",
			Parameters: map[string]*Parameter{
				"param1": {
					Type:        TypeString,
					Description: "test parameter",
				},
			},
		}
		gt.Error(t, spec.Validate())
	})

	t.Run("invalid parameter type", func(t *testing.T) {
		spec := ToolSpec{
			Name:        "test",
			Description: "test description",
			Parameters: map[string]*Parameter{
				"param1": {
					Type:        "invalid",
					Description: "test parameter",
				},
			},
		}
		gt.Error(t, spec.Validate())
	})

	t.Run("required parameter not found", func(t *testing.T) {
		spec := ToolSpec{
			Name:        "test",
			Description: "test description",
			Parameters: map[string]*Parameter{
				"param1": {
					Type:        TypeString,
					Description: "test parameter",
				},
			},
			Required: []string{"param2"},
		}
		gt.Error(t, spec.Validate())
	})

	t.Run("invalid parameter", func(t *testing.T) {
		spec := ToolSpec{
			Name:        "test",
			Description: "test description",
			Parameters: map[string]*Parameter{
				"param1": {
					Type:    TypeNumber,
					Minimum: ptr(10.0),
					Maximum: ptr(1.0),
				},
			},
		}
		gt.Error(t, spec.Validate())
	})

	t.Run("object parameter without properties", func(t *testing.T) {
		spec := ToolSpec{
			Name:        "test",
			Description: "test description",
			Parameters: map[string]*Parameter{
				"param1": {
					Type:        TypeObject,
					Description: "test parameter",
				},
			},
		}
		gt.Error(t, spec.Validate())
	})

	t.Run("array parameter without items", func(t *testing.T) {
		spec := ToolSpec{
			Name:        "test",
			Description: "test description",
			Parameters: map[string]*Parameter{
				"param1": {
					Type:        TypeArray,
					Description: "test parameter",
				},
			},
		}
		gt.Error(t, spec.Validate())
	})
}
