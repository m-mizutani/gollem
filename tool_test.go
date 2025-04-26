package gollam

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
}

func ptr[T any](v T) *T {
	return &v
}
