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

var (
	plannerTmpl    *template.Template
	executorTmpl   *template.Template
	reflectorTmpl  *template.Template
	summarizerTmpl *template.Template
)

func init() {
	plannerTmpl = template.Must(template.New("planner").Parse(plannerPromptTemplate))
	executorTmpl = template.Must(template.New("executor").Parse(executorPromptTemplate))
	reflectorTmpl = template.Must(template.New("reflector").Parse(reflectorPromptTemplate))
	summarizerTmpl = template.Must(template.New("summarizer").Parse(summarizerPromptTemplate))
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

type summarizerTemplateData struct {
	Goal             string
	ExecutionDetails string
	OverallStatus    string
	SystemPrompt     string
}
