package gollem

import (
	"context"
	"encoding/json"

	"github.com/m-mizutani/goerr/v2"
)

const (
	DefaultProceedPrompt = `Review the conversation history carefully to understand what has already been attempted.

Respond in JSON format with the following structure:
{
  "action": "continue|complete",
  "reason": "Brief explanation for the chosen action",
  "next_step": "Specific action to take next (required for continue, you will be called with the next_step prompt)",
  "completion": "Brief summary of what was accomplished (required when action is complete)"
}

Rules:
- Use "continue" ONLY if you have a genuinely NEW and actionable next step that hasn't been tried before
- Use "complete" when analysis is finished, findings are ready, or no new actionable steps remain
- If you notice repetitive patterns in the conversation history, choose "complete" instead
- If you're stuck or can't make meaningful progress, choose "complete"
- When action is "complete", use the 'respond_to_user' function to indicate completion
- Prioritize completion over repetitive attempts`

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

func (x *Facilitation) Validate() error {
	if x.Action != ActionContinue && x.Action != ActionComplete {
		return goerr.New("invalid action")
	}

	switch x.Action {
	case ActionComplete:
		if x.Completion == "" {
			return goerr.New("completion is required when action is complete")
		}
	case ActionContinue:
		if x.NextStep == "" {
			return goerr.New("next_step is required when action is continue")
		}
	}

	return nil
}

// Facilitator is a tool that can be used to control the session loop.
// IsCompleted() is called before calling a method to generate content every loop. If IsCompleted() returns true, the session will be ended.
type Facilitator interface {
	Tool

	// Facilitate is called before calling a method to generate content every loop when there is no next input such as tool results, etc. If Facilitate returns nil, the session will be ended.
	Facilitate(ctx context.Context, history *History) (*Facilitation, error)
}

// DefaultFacilitator is the tool to stop the session loop.
// This tool is used when the agent determines that the session should be ended. The tool name is "respond_to_user".
type defaultFacilitator struct {
	isCompleted bool
	llmClient   LLMClient
	retryLimit  int
}

func newDefaultFacilitator(llmClient LLMClient) Facilitator {
	return &defaultFacilitator{
		llmClient:  llmClient,
		retryLimit: 3,
	}
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
	LoggerFromContext(ctx).Debug("run Facilitate",
		"isComplete", x.isCompleted,
		"history", history,
	)
	if x.isCompleted {
		x.isCompleted = false
		return &Facilitation{
			Action:     ActionComplete,
			Completion: "done",
		}, nil
	}

	// Clone the history to avoid affecting the original session
	clonedHistory := history.Clone()
	ssn, err := x.llmClient.NewSession(ctx,
		WithSessionHistory(clonedHistory),
		WithSessionContentType(ContentTypeJSON),
	)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create session")
	}

	for i := 0; i < x.retryLimit; i++ {
		resp, err := x.updateStatusWithContext(ctx, ssn)
		if err == nil {
			LoggerFromContext(ctx).Debug("facilitated", "facilitation", resp)
			return resp, nil
		}

		LoggerFromContext(ctx).Error("failed to update status", "error", err)
	}

	return nil, goerr.New("failed to facilitate")
}

// updateStatusWithContext generates status with improved prompt
func (x *defaultFacilitator) updateStatusWithContext(ctx context.Context, ssn Session) (*Facilitation, error) {
	output, err := ssn.GenerateContent(ctx, Text("choose your next action or complete"))
	if err != nil {
		return nil, goerr.Wrap(err, "failed to update status")
	}

	if len(output.Texts) == 0 {
		return nil, goerr.New("no response from LLM")
	}

	var resp Facilitation
	if err := json.Unmarshal([]byte(output.Texts[0]), &resp); err != nil {
		return nil, goerr.Wrap(err, "failed to unmarshal response", goerr.V("output", output))
	}

	if err := resp.Validate(); err != nil {
		return nil, goerr.Wrap(err, "invalid response", goerr.V("Facilitation", resp))
	}

	return &resp, nil
}
