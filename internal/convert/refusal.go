package convert

import (
	"github.com/gollem-dev/gollem"
	"github.com/gollem-dev/gollem/trace"
)

// TraceRefusal returns the trace record of r, or nil when r is nil.
func TraceRefusal(r *gollem.Refusal) *trace.Refusal {
	if r == nil {
		return nil
	}
	return &trace.Refusal{
		Reason:      r.Reason,
		Categories:  r.Categories,
		Explanation: r.Explanation,
	}
}
