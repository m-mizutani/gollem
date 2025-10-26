package gollem

import "strings"

// ExecuteResponse represents the final response from Execute method
type ExecuteResponse struct {
	Texts   []string // Response texts array
	History *History // Optional history to append to the main session
}

// NewExecuteResponse creates a new ExecuteResponse with given texts
func NewExecuteResponse(texts ...string) *ExecuteResponse {
	if texts == nil {
		texts = []string{}
	}
	return &ExecuteResponse{
		Texts: texts,
	}
}

// String returns a string representation of the response
func (r *ExecuteResponse) String() string {
	if r == nil || len(r.Texts) == 0 {
		return ""
	}
	return strings.Join(r.Texts, " ")
}

// IsEmpty returns true if the response has no texts or all texts are empty
func (r *ExecuteResponse) IsEmpty() bool {
	if r == nil || len(r.Texts) == 0 {
		return true
	}
	for _, s := range r.Texts {
		if s != "" {
			return false
		}
	}
	return true
}
