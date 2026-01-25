package gollem

import (
	"bytes"
	"context"
	"strings"
	"text/template"

	"github.com/m-mizutani/goerr/v2"
)

// SubAgent represents an agent that can be invoked as a tool by parent agent.
// SubAgent implements the Tool interface, allowing it to be added to an agent's tool list.
type SubAgent struct {
	name        string
	description string
	agent       *Agent

	// Template mode fields (nil when using default query-only mode)
	parsedTemplate *template.Template    // Parsed template (cached)
	parameters     map[string]*Parameter // Parameter schema for LLM
}

// SubAgentOption is the type for options when creating a SubAgent.
type SubAgentOption func(*SubAgent)

// PromptTemplate holds the parsed template and parameter schema for template mode.
// Use NewPromptTemplate to create an instance with proper error handling.
type PromptTemplate struct {
	parsedTemplate *template.Template
	parameters     map[string]*Parameter
}

// NewPromptTemplate creates a new PromptTemplate with the given template and parameters.
// tmpl: Go text/template string to generate the prompt
// params: Parameter schema for LLM (key is parameter name)
//
// The template uses missingkey=error option, so all referenced variables must be provided
// when the template is executed.
//
// Returns an error if the template string is invalid.
func NewPromptTemplate(tmpl string, params map[string]*Parameter) (*PromptTemplate, error) {
	parsed, err := template.New("prompt").Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to parse template")
	}
	return &PromptTemplate{
		parsedTemplate: parsed,
		parameters:     params,
	}, nil
}

// DefaultPromptTemplate returns the default prompt template that accepts a single "query" parameter.
// This is equivalent to the default behavior when no prompt template is specified.
func DefaultPromptTemplate() *PromptTemplate {
	// This template is simple and cannot fail to parse
	parsed, _ := template.New("prompt").Option("missingkey=error").Parse("{{.query}}")
	return &PromptTemplate{
		parsedTemplate: parsed,
		parameters: map[string]*Parameter{
			"query": {
				Type:        TypeString,
				Description: "Natural language query to send to the subagent",
				Required:    true,
			},
		},
	}
}

// Render renders the template with the given arguments and returns the resulting prompt string.
// This method is useful for testing templates independently from the SubAgent.
func (p *PromptTemplate) Render(args map[string]any) (string, error) {
	var buf bytes.Buffer
	if err := p.parsedTemplate.Execute(&buf, args); err != nil {
		return "", goerr.Wrap(err, "failed to execute template")
	}
	return buf.String(), nil
}

// Parameters returns the parameter schema for this template.
// This is useful for inspecting the expected parameters.
func (p *PromptTemplate) Parameters() map[string]*Parameter {
	return p.parameters
}

// WithPromptTemplate sets a pre-configured prompt template for the subagent.
// This option replaces the default "query" parameter behavior.
// When this option is used, the template is rendered with the provided parameters
// and the result is passed to agent.Execute().
func WithPromptTemplate(prompt *PromptTemplate) SubAgentOption {
	return func(s *SubAgent) {
		s.parsedTemplate = prompt.parsedTemplate
		s.parameters = prompt.parameters
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

	if s.parsedTemplate == nil {
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
// In default mode, it uses the default prompt template with a "query" parameter.
// In template mode, it renders the template with the arguments and passes the result to the agent.
func (s *SubAgent) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	var pt *PromptTemplate
	if s.parsedTemplate == nil {
		// Default mode: use default prompt template
		pt = DefaultPromptTemplate()
	} else {
		// Template mode: use custom template
		pt = &PromptTemplate{
			parsedTemplate: s.parsedTemplate,
			parameters:     s.parameters,
		}
	}

	prompt, err := pt.Render(args)
	if err != nil {
		return nil, err
	}

	// Execute the child agent
	resp, err := s.agent.Execute(ctx, Text(prompt))
	if err != nil {
		return nil, goerr.Wrap(err, "subagent execution failed")
	}

	// Build response text from ExecuteResponse
	var responseText string
	if resp != nil && len(resp.Texts) > 0 {
		responseText = strings.Join(resp.Texts, "\n")
	}

	return map[string]any{
		"response": responseText,
		"status":   "success",
	}, nil
}
