package gollem

import (
	_ "embed"
	"text/template"
)

//go:embed templates/planner_prompt.md
var plannerPromptTemplate string

//go:embed templates/executor_prompt.md
var executorPromptTemplate string

//go:embed templates/reflector_prompt.md
var reflectorPromptTemplate string

//go:embed templates/summarizer_prompt.md
var summarizerPromptTemplate string

//go:embed templates/goal_interpreter_prompt.md
var goalInterpreterPromptTemplate string

var (
	plannerTmpl         *template.Template
	executorTmpl        *template.Template
	reflectorTmpl       *template.Template
	summarizerTmpl      *template.Template
	goalInterpreterTmpl *template.Template
)

func init() {
	plannerTmpl = template.Must(template.New("planner").Parse(plannerPromptTemplate))
	executorTmpl = template.Must(template.New("executor").Parse(executorPromptTemplate))
	reflectorTmpl = template.Must(template.New("reflector").Parse(reflectorPromptTemplate))
	summarizerTmpl = template.Must(template.New("summarizer").Parse(summarizerPromptTemplate))
	goalInterpreterTmpl = template.Must(template.New("goalInterpreter").Parse(goalInterpreterPromptTemplate))
}

type plannerTemplateData struct {
	ToolInfo     string
	Goal         string
	SystemPrompt string
}

type executorTemplateData struct {
	Intent          string
	ProgressSummary string
	SystemPrompt    string
}

type reflectorTemplateData struct {
	Goal              string
	InterpretedGoal   string
	CurrentPlanStatus string
	OriginalPlan      string
	CompletedSteps    string
	LastStepResult    string
	SystemPrompt      string
}

type summarizerTemplateData struct {
	Goal             string
	InterpretedGoal  string
	ExecutionDetails string
	OverallStatus    string
	SystemPrompt     string
}

type goalInterpreterTemplateData struct {
	UserInput    string
	SystemPrompt string
}
