package gollem

import (
	"bytes"
	"context"
	"text/template"

	"github.com/m-mizutani/goerr/v2"
)

// SubAgent represents an agent that can be invoked as a tool by parent agent.
// SubAgent implements the Tool interface, allowing it to be added to an agent's tool list.
type SubAgent struct {
	name        string
	description string
	agent       *Agent

	// Template mode fields (nil/empty when using default query-only mode)
	template   string                // Prompt template
	parameters map[string]*Parameter // Parameter schema for LLM
}

// SubAgentOption is the type for options when creating a SubAgent.
type SubAgentOption func(*SubAgent)

// WithPromptTemplate sets custom parameters and template for prompt generation.
// template: Go text/template string to generate the prompt
// params: Parameter schema for LLM (key is parameter name)
//
// This option replaces the default "query" parameter behavior.
// When this option is used, the template is rendered with the provided parameters
// and the result is passed to agent.Execute().
func WithPromptTemplate(tmpl string, params map[string]*Parameter) SubAgentOption {
	return func(s *SubAgent) {
		s.template = tmpl
		s.parameters = params
	}
}

// NewSubAgent creates a new SubAgent that wraps an existing Agent.
// name: Tool name for the subagent (required, used by LLM to invoke)
// description: Description of what this subagent does (required, helps LLM decide when to use)
// agent: The Agent instance to execute (required)
func NewSubAgent(name, description string, agent *Agent, opts ...SubAgentOption) *SubAgent {
	s := &SubAgent{
		name:        name,
		description: description,
		agent:       agent,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Spec returns the ToolSpec for this SubAgent.
// In default mode, it returns a spec with a single "query" parameter.
// In template mode, it returns a spec with the custom parameters.
func (s *SubAgent) Spec() ToolSpec {
	var params map[string]*Parameter

	if s.template == "" {
		// Default mode: query only
		params = map[string]*Parameter{
			"query": {
				Type:        TypeString,
				Description: "Natural language query to send to the subagent",
				Required:    true,
			},
		}
	} else {
		// Template mode: use custom parameters
		params = s.parameters
	}

	return ToolSpec{
		Name:        s.name,
		Description: s.description,
		Parameters:  params,
	}
}

// Run executes the SubAgent with the given arguments.
// In default mode, it extracts the "query" parameter and passes it to the agent.
// In template mode, it renders the template with the arguments and passes the result to the agent.
func (s *SubAgent) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	var prompt string

	if s.template == "" {
		// Default mode: extract query parameter
		queryVal, ok := args["query"]
		if !ok {
			return map[string]any{
				"response": "",
				"status":   "error",
			}, goerr.Wrap(ErrInvalidParameter, "query parameter is required")
		}

		query, ok := queryVal.(string)
		if !ok {
			return map[string]any{
				"response": "",
				"status":   "error",
			}, goerr.Wrap(ErrInvalidParameter, "query parameter must be a string")
		}

		prompt = query
	} else {
		// Template mode: render template with arguments
		tmpl, err := template.New("subagent").Parse(s.template)
		if err != nil {
			return map[string]any{
				"response": "",
				"status":   "error",
			}, goerr.Wrap(err, "failed to parse template")
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, args); err != nil {
			return map[string]any{
				"response": "",
				"status":   "error",
			}, goerr.Wrap(err, "failed to execute template")
		}

		prompt = buf.String()
	}

	// Execute the child agent
	resp, err := s.agent.Execute(ctx, Text(prompt))
	if err != nil {
		return map[string]any{
			"response": "",
			"status":   "error",
		}, goerr.Wrap(err, "subagent execution failed")
	}

	// Build response text from ExecuteResponse
	var responseText string
	if resp != nil && len(resp.Texts) > 0 {
		for i, text := range resp.Texts {
			if i > 0 {
				responseText += "\n"
			}
			responseText += text
		}
	}

	return map[string]any{
		"response": responseText,
		"status":   "success",
	}, nil
}
