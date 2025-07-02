package gollem

import (
	"context"
	"encoding/json"

	"github.com/m-mizutani/goerr/v2"
)

const (
	DefaultProceedPrompt = `Respond in JSON format with the following structure:
{
  "action": "continue|complete",
  "reason": "Brief explanation for the chosen action",
  "next_step": "Specific action to take next (only for continue, you will be called with the next_step prompt)",
  "completion": "Brief summary of what was accomplished (only for complete)"
}

Rules:
- Use "continue" only if you have a specific, actionable next step
- Use "complete" when analysis is finished and findings are ready
- When action is "complete", use the 'respond_to_user' function to indicate completion`
	DefaultFacilitatorName = "respond_to_user"
)

type ActionType string

const (
	ActionContinue ActionType = "continue"
	ActionComplete ActionType = "complete"
)

type Facilitation struct {
	Action     ActionType `json:"action"`
	Reason     string     `json:"reason"`
	NextStep   string     `json:"next_step,omitempty"`
	Completion string     `json:"completion,omitempty"`
}

// Facilitator is a tool that can be used to control the session loop.
// IsCompleted() is called before calling a method to generate content every loop. If IsCompleted() returns true, the session will be ended.
type Facilitator interface {
	Tool
	IsCompleted() bool

	// Facilitate is called before calling a method to generate content every loop when there is no next input such as tool results, etc. If Facilitate returns nil, the session will be ended.
	Facilitate(ctx context.Context, history *History) (*Facilitation, error)
}

// DefaultFacilitator is the tool to stop the session loop.
// This tool is used when the agent determines that the session should be ended. The tool name is "respond_to_user".
type defaultFacilitator struct {
	isCompleted bool
	llmClient   LLMClient
}

func newDefaultFacilitator(llmClient LLMClient) Facilitator {
	return &defaultFacilitator{llmClient: llmClient}
}

var _ Facilitator = &defaultFacilitator{}

// Spec for Tool interface
func (x *defaultFacilitator) Spec() ToolSpec {
	return ToolSpec{
		Name:        DefaultFacilitatorName,
		Description: "Call this tool when you have gathered all necessary information, completed all required actions, and already provided the final answer to the user's original request. This signals that your work on the current request is finished.",
		Parameters: map[string]*Parameter{
			"summary": {
				Type:        TypeString,
				Description: "Brief summary of what was accomplished",
			},
		},
	}
}

// Run for Tool interface
func (x *defaultFacilitator) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	x.isCompleted = true
	return nil, nil
}

// IsCompleted for Facilitator interface
func (x *defaultFacilitator) IsCompleted() bool {
	return x.isCompleted
}

// UpdateStatus for Facilitator interface
func (x *defaultFacilitator) Facilitate(ctx context.Context, history *History) (*Facilitation, error) {
	ssn, err := x.llmClient.NewSession(ctx,
		WithSessionHistory(history),
		WithSessionContentType(ContentTypeJSON),
	)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create session")
	}

	output, err := ssn.GenerateContent(ctx, Text(DefaultProceedPrompt))
	if err != nil {
		return nil, goerr.Wrap(err, "failed to update status")
	}

	if len(output.Texts) == 0 {
		return nil, goerr.New("no response from LLM")
	}

	var resp Facilitation
	if err := json.Unmarshal([]byte(output.Texts[0]), &resp); err != nil {
		return nil, goerr.Wrap(err, "failed to unmarshal response")
	}

	return &resp, nil
}
