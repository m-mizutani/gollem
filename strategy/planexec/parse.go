package planexec

import (
	"fmt"
	"strings"

	"github.com/m-mizutani/gollem"
)

// parseTaskResult extracts the task execution result from LLM response
func parseTaskResult(response *gollem.Response) string {
	if response == nil {
		return ""
	}

	// Combine all text responses
	var results []string
	for _, text := range response.Texts {
		text = strings.TrimSpace(text)
		if text != "" {
			results = append(results, text)
		}
	}

	// If there are function calls, include them as well
	if len(response.FunctionCalls) > 0 {
		results = append(results, "Tool calls executed:")
		for _, fc := range response.FunctionCalls {
			results = append(results, "- "+fc.Name)
			// Include arguments if any
			if len(fc.Arguments) > 0 {
				for key, val := range fc.Arguments {
					results = append(results, fmt.Sprintf("  %s: %v", key, val))
				}
			}
		}
	}

	return strings.Join(results, "\n")
}
