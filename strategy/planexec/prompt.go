package planexec

import (
	"context"
	"fmt"
	"strings"

	"github.com/m-mizutani/gollem"
)

const planPromptTemplate = `You are a helpful assistant that analyzes user requests and creates execution plans when needed.

Analyze the user's request and determine if it requires a step-by-step plan or can be answered directly.

If the request is simple and can be answered immediately (like a question, greeting, or simple calculation), respond directly without creating a plan.

If the request requires multiple steps or complex execution, create a structured plan with clear tasks.

User request: %s

Respond in the following JSON format:

For direct response (no plan needed):
{
  "needs_plan": false,
  "direct_response": "Your direct answer here"
}

For planned execution:
{
  "needs_plan": true,
  "goal": "The overall goal",
  "tasks": [
    {
      "description": "First task description"
    },
    {
      "description": "Second task description"
    }
  ]
}

Think step by step and provide your response:`

const executePromptTemplate = `You are a task executor that can ONLY use function/tool calls to complete tasks.

Overall Goal: %s
Current Task: %s

Previous completed tasks:
%s

CRITICAL: You do NOT have access to any information or data except through function calls.
You MUST call the appropriate function/tool to execute this task.
Do NOT respond with text. Your response MUST be a function call.
If you respond with text instead of a function call, the system will fail.`

const reflectPromptTemplate = `You have just completed a task. Review the progress and determine next steps.

Overall Goal: %s

Completed Tasks:
%s

Remaining Tasks:
%s

Latest Task Result:
%s

Based on the progress so far:
1. Have we achieved the overall goal?
2. Should we continue with remaining tasks?
3. Do we need to modify the plan?

Respond in JSON format:
{
  "goal_achieved": true/false,
  "should_continue": true/false,
  "reason": "Explanation of your decision",
  "plan_updates": {
    "new_tasks": ["description of any new tasks if needed"],
    "remove_task_ids": ["IDs of any tasks to remove"]
  }
}

Note: Use task IDs (not descriptions) when specifying tasks to remove.`

// buildPlanPrompt creates a prompt for analyzing and planning
func buildPlanPrompt(_ context.Context, inputs []gollem.Input) []gollem.Input {
	// Combine all input texts
	var inputTexts []string
	for _, input := range inputs {
		if text, ok := input.(gollem.Text); ok {
			inputTexts = append(inputTexts, string(text))
		}
	}

	userRequest := strings.Join(inputTexts, " ")
	prompt := fmt.Sprintf(planPromptTemplate, userRequest)

	return []gollem.Input{gollem.Text(prompt)}
}

// buildExecutePrompt creates a prompt for executing a specific task
func buildExecutePrompt(ctx context.Context, task *Task, plan *Plan) []gollem.Input {
	// Build list of completed tasks
	var completedTasks []string
	for _, t := range plan.Tasks {
		if t.State == TaskStateCompleted {
			completedTasks = append(completedTasks, fmt.Sprintf("[ID: %s] %s", t.ID, t.Description))
			if t.Result != "" {
				completedTasks = append(completedTasks, fmt.Sprintf("   Result: %s", t.Result))
			}
		}
	}

	completedStr := "None"
	if len(completedTasks) > 0 {
		completedStr = strings.Join(completedTasks, "\n")
	}

	prompt := fmt.Sprintf(executePromptTemplate, plan.Goal, task.Description, completedStr)

	return []gollem.Input{gollem.Text(prompt)}
}

// buildReflectPrompt creates a prompt for reflection after task completion
func buildReflectPrompt(ctx context.Context, plan *Plan) []gollem.Input {
	// Build completed tasks list
	var completedTasks []string
	var remainingTasks []string
	var latestResult string

	for _, task := range plan.Tasks {
		taskStr := fmt.Sprintf("[ID: %s] %s", task.ID, task.Description)

		switch task.State {
		case TaskStateCompleted:
			completedTasks = append(completedTasks, taskStr)
			if task.Result != "" {
				latestResult = task.Result // Keep track of the latest result
			}
		case TaskStatePending:
			remainingTasks = append(remainingTasks, taskStr)
		}
	}

	completedStr := strings.Join(completedTasks, "\n")
	if completedStr == "" {
		completedStr = "None"
	}

	remainingStr := strings.Join(remainingTasks, "\n")
	if remainingStr == "" {
		remainingStr = "None"
	}

	prompt := fmt.Sprintf(reflectPromptTemplate, plan.Goal, completedStr, remainingStr, latestResult)

	return []gollem.Input{gollem.Text(prompt)}
}
