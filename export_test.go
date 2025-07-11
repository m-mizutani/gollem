package gollem

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
