package gollem

import (
	"log/slog"
	"os"
)

var NewDefaultFacilitator = newDefaultFacilitator

// Plan internal types and methods for testing
type (
	TestPlanToDo       = planToDo
	TestPlanReflection = planReflection
	TestPlanToDoChange = PlanToDoChange
)

// Plan internal constants for testing
const (
	TestPlanToDoChangeTypeAdded   = PlanToDoChangeTypeAdded
	TestPlanToDoChangeTypeUpdated = PlanToDoChangeTypeUpdated
	TestPlanToDoChangeTypeRemoved = PlanToDoChangeTypeRemoved
)

var CtxWithLogger = ctxWithLogger

// Plan internal field accessors for testing
func (p *Plan) TestGetTodos() []planToDo {
	return p.todos
}

func (p *Plan) TestSetTodos(todos []planToDo) {
	p.todos = todos
}

func (p *Plan) TestUpdatePlan(reflection *planReflection) error {
	return p.updatePlan(reflection)
}

// Helper to create a test plan
func NewTestPlan(id string, input string, todos []planToDo) *Plan {
	return &Plan{
		id:    id,
		input: input,
		todos: todos,
	}
}

// Helper to create plan reflection
func NewTestPlanReflection(reflType PlanReflectionType, newTodos []planToDo) *planReflection {
	return &planReflection{
		Type:     reflType,
		NewToDos: newTodos,
	}
}

var debugLogger *slog.Logger

func init() {
	debugLogger = slog.New(slog.DiscardHandler)
	if _, ok := os.LookupEnv("GOLLEM_DEBUG"); ok {
		debugLogger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		}))
	}
}

func DebugLogger() *slog.Logger { return debugLogger }
