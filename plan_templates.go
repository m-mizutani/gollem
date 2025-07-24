package gollem

import (
	_ "embed"
	"text/template"
)

// templates

//go:embed templates/planner_prompt.md
var plannerPromptTemplate string

//go:embed templates/executor_prompt.md
var executorPromptTemplate string

//go:embed templates/reflector_prompt.md
var reflectorPromptTemplate string

//go:embed templates/summarizer_prompt.md
var summarizerPromptTemplate string

//go:embed templates/goal_clarifier_prompt.md
var goalClarifierPromptTemplate string

var (
	plannerTmpl       *template.Template
	executorTmpl      *template.Template
	reflectorTmpl     *template.Template
	summarizerTmpl    *template.Template
	goalClarifierTmpl *template.Template
)

func init() {
	plannerTmpl = template.Must(template.New("planner").Parse(plannerPromptTemplate))
	executorTmpl = template.Must(template.New("executor").Parse(executorPromptTemplate))
	reflectorTmpl = template.Must(template.New("reflector").Parse(reflectorPromptTemplate))
	summarizerTmpl = template.Must(template.New("summarizer").Parse(summarizerPromptTemplate))
	goalClarifierTmpl = template.Must(template.New("goalClarifier").Parse(goalClarifierPromptTemplate))
}

type plannerTemplateData struct {
	ToolInfo string
	Goal     string
	Language string
}

type executorTemplateData struct {
	Intent              string
	ProgressSummary     string
	Language            string
	CurrentIteration    int
	MaxIterations       int
	RemainingIterations int
}

type reflectorTemplateData struct {
	Goal                   string
	ClarifiedGoal          string
	CurrentPlanStatus      string
	OriginalPlan           string
	PendingTodos           string
	CompletedSteps         string
	LastStepResult         string
	SimplifiedSystemPrompt string
	Language               string
	IterationLimitInfo     string
}

type summarizerTemplateData struct {
	Goal             string
	ClarifiedGoal    string
	ExecutionDetails string
	OverallStatus    string
	SystemPrompt     string
	Language         string
}

type goalClarifierTemplateData struct {
	UserInput    string
	SystemPrompt string
	Language     string
}
