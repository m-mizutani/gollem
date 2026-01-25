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
// The template uses missingkey=zero option, so missing variables are replaced with
// their zero value (empty string for strings). This allows conditional checks like
// {{if .optional}}...{{end}} for optional parameters.
//
// Returns an error if the template string is invalid.
func NewPromptTemplate(tmpl string, params map[string]*Parameter) (*PromptTemplate, error) {
	parsed, err := template.New("prompt").Option("missingkey=zero").Parse(tmpl)
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
	parsed := template.Must(template.New("prompt").Option("missingkey=zero").Parse("{{.query}}"))
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
// Returns an error if validation fails (missing required params, type mismatch, constraint violations).
// Missing optional parameters are replaced with their zero value (empty string for strings).
func (p *PromptTemplate) Render(args map[string]any) (string, error) {
	// Validate all parameters and build data map
	data := make(map[string]any, len(p.parameters))
	for name, param := range p.parameters {
		value := args[name]
		if err := param.ValidateValue(name, value); err != nil {
			return "", err
		}
		if value != nil {
			data[name] = value
		} else {
			// Set zero value for missing optional parameters to avoid <no value>
			data[name] = zeroValue(param.Type)
		}
	}

	var buf bytes.Buffer
	if err := p.parsedTemplate.Execute(&buf, data); err != nil {
		return "", goerr.Wrap(err, "failed to execute template")
	}
	return buf.String(), nil
}

// zeroValue returns the zero value for a given parameter type.
func zeroValue(t ParameterType) any {
	switch t {
	case TypeString:
		return ""
	case TypeNumber, TypeInteger:
		return 0
	case TypeBoolean:
		return false
	case TypeArray:
		return []any{}
	case TypeObject:
		return map[string]any{}
	default:
		return ""
	}
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
