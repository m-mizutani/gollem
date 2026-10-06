package convert_test

import (
	"testing"

	"github.com/gollem-dev/gollem"
	"github.com/gollem-dev/gollem/internal/convert"
	"github.com/gollem-dev/gollem/trace"
	"github.com/m-mizutani/gt"
)

func TestTraceRefusal(t *testing.T) {
	t.Run("nil stays nil", func(t *testing.T) {
		gt.Nil(t, convert.TraceRefusal(nil))
	})

	t.Run("copies every value", func(t *testing.T) {
		got := convert.TraceRefusal(&gollem.Refusal{
			Reason:      "refusal",
			Categories:  []string{"cyber"},
			Explanation: "the request could enable cyber harm",
		})
		gt.Equal(t, &trace.Refusal{
			Reason:      "refusal",
			Categories:  []string{"cyber"},
			Explanation: "the request could enable cyber harm",
		}, got)
	})
}
