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
				},
			},
			Required: []string{"param1"},
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

	t.Run("required parameter not found", func(t *testing.T) {
		spec := gollem.ToolSpec{
			Name:        "test",
			Description: "test description",
			Parameters: map[string]*gollem.Parameter{
				"param1": {
					Type:        gollem.TypeString,
					Description: "test parameter",
				},
			},
			Required: []string{"param2"},
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
