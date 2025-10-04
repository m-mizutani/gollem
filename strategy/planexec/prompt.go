package planexec

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
)

//go:embed prompts/plan.md
var planPromptTemplate string

//go:embed prompts/execute.md
var executePromptTemplate string

//go:embed prompts/reflect.md
var reflectPromptTemplate string

// buildPlanPrompt creates a prompt for analyzing and planning
func buildPlanPrompt(_ context.Context, inputs []gollem.Input, tools []gollem.Tool) []gollem.Input {
	// Combine all input texts
	var inputTexts []string
	for _, input := range inputs {
		if text, ok := input.(gollem.Text); ok {
			inputTexts = append(inputTexts, string(text))
		}
	}

	userRequest := strings.Join(inputTexts, " ")

	// Build tool list
	toolList := buildToolList(tools)

	tmpl, err := template.New("plan").Parse(planPromptTemplate)
	if err != nil {
		panic(goerr.Wrap(err, "failed to parse plan template"))
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]interface{}{
		"UserRequest": userRequest,
		"ToolList":    toolList,
	}); err != nil {
		panic(goerr.Wrap(err, "failed to execute plan template"))
	}

	return []gollem.Input{gollem.Text(buf.String())}
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

	tmpl, err := template.New("execute").Parse(executePromptTemplate)
	if err != nil {
		panic(goerr.Wrap(err, "failed to parse execute template"))
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]interface{}{
		"Goal":            plan.Goal,
		"TaskDescription": task.Description,
		"CompletedTasks":  completedStr,
	}); err != nil {
		panic(goerr.Wrap(err, "failed to execute execute template"))
	}

	return []gollem.Input{gollem.Text(buf.String())}
}

// buildReflectPrompt creates a prompt for reflection after task completion
func buildReflectPrompt(ctx context.Context, plan *Plan, tools []gollem.Tool) []gollem.Input {
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

	// Build tool list
	toolList := buildToolList(tools)

	tmpl, err := template.New("reflect").Parse(reflectPromptTemplate)
	if err != nil {
		panic(goerr.Wrap(err, "failed to parse reflect template"))
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]interface{}{
		"Goal":           plan.Goal,
		"CompletedTasks": completedStr,
		"RemainingTasks": remainingStr,
		"LatestResult":   latestResult,
		"ToolList":       toolList,
	}); err != nil {
		panic(goerr.Wrap(err, "failed to execute reflect template"))
	}

	return []gollem.Input{gollem.Text(buf.String())}
}

// buildToolList creates a formatted list of available tools
func buildToolList(tools []gollem.Tool) string {
	if len(tools) == 0 {
		return "No tools available"
	}

	var toolDescriptions []string
	for _, tool := range tools {
		spec := tool.Spec()
		toolDesc := fmt.Sprintf("- **%s**: %s", spec.Name, spec.Description)

		// Add parameter information if available
		if len(spec.Parameters) > 0 {
			var params []string
			for paramName := range spec.Parameters {
				params = append(params, paramName)
			}
			if len(params) > 0 {
				toolDesc += fmt.Sprintf("\n  Parameters: %s", strings.Join(params, ", "))
			}
		}

		toolDescriptions = append(toolDescriptions, toolDesc)
	}

	return strings.Join(toolDescriptions, "\n")
}
