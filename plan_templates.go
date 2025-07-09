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

var (
	plannerTmpl   *template.Template
	executorTmpl  *template.Template
	reflectorTmpl *template.Template
)

func init() {
	plannerTmpl = template.Must(template.New("planner").Parse(plannerPromptTemplate))
	executorTmpl = template.Must(template.New("executor").Parse(executorPromptTemplate))
	reflectorTmpl = template.Must(template.New("reflector").Parse(reflectorPromptTemplate))
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
	CurrentPlanStatus string
	OriginalPlan      string
	CompletedSteps    string
	LastStepResult    string
	SystemPrompt      string
}
