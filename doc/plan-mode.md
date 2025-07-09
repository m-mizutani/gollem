# Plan Mode

Plan mode is a sophisticated execution framework within gollem that enables multi-step task planning and execution. It transforms a high-level user goal into a structured sequence of actionable steps that are executed with reflection and adaptation capabilities.

## Overview

Plan mode addresses complex workflows that require breaking down a high-level goal into multiple coordinated steps. Unlike regular agent interactions that handle immediate requests, plan mode creates a structured execution plan that can adapt based on intermediate results.

### Key Features

- **Multi-step Planning**: Automatically breaks down complex goals into actionable steps
- **Dynamic Adaptation**: Plans can be modified during execution based on results
- **Progress Tracking**: Comprehensive hooks and status tracking for monitoring execution
- **Persistence**: Plans can be serialized and restored for stateless applications
- **Tool Integration**: Works seamlessly with any gollem tools including MCP servers

## Core Concepts

### Plan Structure

A plan consists of:
- **Input**: The original user goal
- **ToDos**: A sequence of structured steps with descriptions and intents
- **State**: Current execution state (created, running, completed, failed)
- **Session**: Independent session that maintains conversation history

### PlanToDo Structure (External Reference)

The public `PlanToDo` structure provides access to plan step information:

```go
type PlanToDo struct {
    ID          string              // Unique identifier
    Description string              // Detailed description
    Intent      string              // High-level intention
    Status      string              // Current status (Pending, Executing, Completed, Failed, Skipped)
    Completed   bool                // Whether the todo is completed
    Error       error               // Any error that occurred
    Result      *PlanToDoResult     // Execution results (if completed)
}

type PlanToDoResult struct {
    Output     string              // Text output from execution
    ToolCalls  []*FunctionCall     // Tool calls made during execution
    Data       map[string]any      // Tool execution results
    ExecutedAt time.Time           // When the todo was executed
}
```

## Basic Usage

### Creating and Executing a Plan

```go
package main

import (
    "context"
    "fmt"
    "github.com/m-mizutani/gollem"
    "github.com/m-mizutani/gollem/llm/openai"
)

func main() {
    // Create LLM client
    client := openai.New()

    // Create agent with tools
    agent := gollem.New(client,
        gollem.WithTools(&SearchTool{}, &AnalysisTool{}),
    )

    // Create plan
    plan, err := agent.Plan(context.Background(),
        "Research the latest trends in AI and create a summary report")
    if err != nil {
        panic(err)
    }

    // Execute plan
    result, err := plan.Execute(context.Background())
    if err != nil {
        panic(err)
    }

    fmt.Println("Plan completed:", result)
}
```

### Plan Serialization

Plans can be serialized for persistence and later restoration:

```go
// Serialize plan
data, err := plan.Serialize()
if err != nil {
    panic(err)
}

// Save to file or database
err = saveToStorage(data)
if err != nil {
    panic(err)
}

// Later, restore the plan
data, err = loadFromStorage()
if err != nil {
    panic(err)
}

restoredPlan, err := agent.NewPlanFromData(ctx, data)
if err != nil {
    panic(err)
}

// Continue execution
result, err := restoredPlan.Execute(context.Background())

// Access the plan's independent session with complete history
session := restoredPlan.Session()
planHistory := session.History()
```

## Plan Execution Workflow

Plan execution follows a sophisticated three-phase loop for each step:

### Phase 1: Step Execution
- **Executor Session**: Executes the current todo using available tools
- Provides context about the current intent and progress summary
- Returns tool calls and execution results
- Updates todo status and captures results

### Phase 2: Reflection
- **Reflector Session**: Analyzes progress and determines next actions
- Evaluates whether the goal has been achieved
- Can modify, add, or remove remaining todos based on results
- Decides if plan should continue or complete

### Phase 3: Plan Updates
- Updates plan state based on reflection results
- Merges new todos or modifications into the plan
- Continues to next pending todo or completes plan

## Configuration Options

### Plan Creation Options

```go
plan, err := agent.Plan(context.Background(), "task description",
    gollem.WithPlanSystemPrompt("Custom prompt"),   // Custom system prompt
    gollem.WithPlanHistory(existingHistory),        // Provide initial history
)
```

### Progress Monitoring Hooks

```go
plan, err := agent.Plan(context.Background(), "task description",
    gollem.WithPlanCreatedHook(func(ctx context.Context, plan *gollem.Plan) error {
        fmt.Println("Plan created with", len(plan.GetToDos()), "steps")
        return nil
    }),
    gollem.WithPlanCompletedHook(func(ctx context.Context, plan *gollem.Plan, result string) error {
        fmt.Println("Plan completed:", result)
        return nil
    }),
)
```

### Step-Level Hooks

```go
plan, err := agent.Plan(context.Background(), "task description",
    gollem.WithPlanToDoStartHook(func(ctx context.Context, plan *gollem.Plan, todo gollem.PlanToDo) error {
        fmt.Printf("Starting: %s\n", todo.Description)
        return nil
    }),
    gollem.WithPlanToDoCompletedHook(func(ctx context.Context, plan *gollem.Plan, todo gollem.PlanToDo) error {
        fmt.Printf("Completed: %s (Status: %s)\n", todo.Description, todo.Status)
        return nil
    }),
    gollem.WithPlanToDoUpdatedHook(func(ctx context.Context, plan *gollem.Plan, changes []gollem.PlanToDoChange) error {
        fmt.Printf("Plan updated with %d changes\n", len(changes))
        for _, change := range changes {
            switch change.Type {
            case gollem.PlanToDoChangeUpdated:
                fmt.Printf("  Updated: %s\n", change.Description)
            case gollem.PlanToDoChangeAdded:
                fmt.Printf("  Added: %s\n", change.Description)
            case gollem.PlanToDoChangeRemoved:
                fmt.Printf("  Removed: %s\n", change.Description)
            }
        }
        return nil
    }),
    gollem.WithPlanMessageHook(func(ctx context.Context, plan *gollem.Plan, message gollem.PlanExecutionMessage) error {
        fmt.Printf("Message [%s]: %s\n", message.Type, message.Content)
        return nil
    }),
)
```

## Advanced Features

### Enhanced Progress Tracking

Plan mode now provides detailed tracking of plan refinements and execution messages:

#### Plan ToDo Updates
When reflection occurs, the plan may be updated with:
- **Refined ToDos**: Existing tasks that have been modified based on new insights
- **New ToDos**: Additional tasks discovered during execution
- **Removed ToDos**: Tasks that are no longer needed

Use `WithPlanToDoUpdatedHook` to track these changes:

```go
gollem.WithPlanToDoUpdatedHook(func(ctx context.Context, plan *gollem.Plan, changes []gollem.PlanToDoChange) error {
    for _, change := range changes {
        switch change.Type {
        case gollem.PlanToDoChangeUpdated:
            fmt.Printf("Refined: %s -> %s\n", change.OldToDo.Description, change.NewToDo.Description)
        case gollem.PlanToDoChangeAdded:
            fmt.Printf("Added: %s\n", change.NewToDo.Description)
        case gollem.PlanToDoChangeRemoved:
            fmt.Printf("Removed: %s\n", change.OldToDo.Description)
        }
    }
    return nil
})
```

#### Execution Messages
Plan execution generates various types of messages that can be captured:

- **Response Messages**: LLM responses during step execution
- **System Messages**: Internal system notifications
- **Action Messages**: Actions being taken
- **Thought Messages**: LLM reasoning and thinking

Use `WithPlanMessageHook` to capture these messages:

```go
gollem.WithPlanMessageHook(func(ctx context.Context, plan *gollem.Plan, message gollem.PlanExecutionMessage) error {
    switch message.Type {
    case gollem.PlanMessageResponse:
        fmt.Printf("[%s] Response: %s\n", message.TodoID, message.Content)
    case gollem.PlanMessageSystem:
        fmt.Printf("[%s] System: %s\n", message.TodoID, message.Content)
    case gollem.PlanMessageAction:
        fmt.Printf("[%s] Action: %s\n", message.TodoID, message.Content)
    case gollem.PlanMessageThought:
        fmt.Printf("[%s] Thought: %s\n", message.TodoID, message.Content)
    }
    return nil
})
```

#### Reflection Types
Plan reflection now categorizes the type of reflection that occurred:

- **Continue**: Plan continues with current todos
- **Refine**: Todos were refined/updated
- **Expand**: New todos were added
- **Complete**: Plan completed by reflection
- **Refined Done**: Plan completed after refinement

### Session History Management

Plan mode maintains conversation history through independent session management:
- **Independent Sessions**: Plan sessions are completely independent from Agent sessions
- **Plan-Specific Context**: Each plan maintains its own conversation context and history
- **Session Access**: Use `plan.Session()` to access the plan's session and history
- **History Initialization**: Use `WithPlanHistory()` option to provide initial history when creating plans
- **Thread Safety**: Plan sessions are not thread-safe; use one plan execution per goroutine
- **Session Isolation**: Plan execution does not affect Agent's session state
- **Session Requirement**: Plans require a valid session; execution will fail with `ErrPlanNotInitialized` if session is nil
- **Session as Source of Truth**: The plan's session is the authoritative source for conversation history

### Dynamic Plan Modification

During reflection, plans can be dynamically modified:
- **Add new steps**: Based on discoveries during execution
- **Remove steps**: When they become unnecessary
- **Modify existing steps**: Update descriptions or intents

### Error Handling

Plan mode provides specific error types:

```go
result, err := plan.Execute(context.Background())
if err != nil {
    switch err {
    case gollem.ErrPlanAlreadyExecuted:
        fmt.Println("Plan has already been executed")
    case gollem.ErrPlanNotInitialized:
        fmt.Println("Plan is missing required components (agent or session)")
    default:
        fmt.Printf("Plan execution failed: %v\n", err)
    }
}
```

**Common initialization errors**:
- `ErrPlanNotInitialized`: Occurs when the plan's agent or session is nil
  - When using direct JSON unmarshaling without `Agent.NewPlanFromData()`
  - When the plan session is not properly initialized during creation or restoration

### Integration with Facilitator

Plans can integrate with the facilitator system for controlled termination:

```go
// Tools can signal conversation exit
func (t *MyTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
    if shouldExit {
        return nil, gollem.ErrExitConversation
    }
    // ... normal execution
}
```

## Best Practices

### Tool Design for Plan Mode

Design tools with plan mode in mind:

```go
type ResearchTool struct{}

func (t *ResearchTool) Spec() gollem.ToolSpec {
    return gollem.ToolSpec{
        Name:        "research",
        Description: "Research information on a specific topic",
        Parameters: map[string]*gollem.Parameter{
            "topic": {
                Type:        gollem.TypeString,
                Description: "Topic to research",
            },
            "depth": {
                Type:        gollem.TypeString,
                Description: "Research depth: surface, moderate, deep",
                Default:     "moderate",
            },
        },
        Required: []string{"topic"},
    }
}

func (t *ResearchTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
    topic := args["topic"].(string)
    depth := args["depth"].(string)

    // Perform research based on parameters
    results := performResearch(topic, depth)

    return map[string]any{
        "findings": results,
        "sources":  getSources(),
        "summary":  generateSummary(results),
    }, nil
}
```

### Progress Monitoring

Use hooks to provide real-time feedback:

```go
func createProgressMonitor() gollem.PlanOption {
    return gollem.WithPlanToDoStartHook(func(ctx context.Context, plan *gollem.Plan, todo gollem.PlanToDo) error {
        fmt.Printf("🔄 %s\n", todo.Description)
        return nil
    })
}

func createCompletionMonitor() gollem.PlanOption {
    return gollem.WithPlanToDoCompletedHook(func(ctx context.Context, plan *gollem.Plan, todo gollem.PlanToDo) error {
        status := "✅"
        if todo.Status == "Failed" {
            status = "❌"
        } else if todo.Status == "Skipped" {
            status = "⏭️"
        }
        fmt.Printf("%s %s\n", status, todo.Description)
        return nil
    })
}
```

### Accessing Plan Progress

Access plan progress using the external reference approach:

```go
// Get all todos with their current status
todos := plan.GetToDos()
for _, todo := range todos {
    fmt.Printf("Todo: %s, Status: %s\n", todo.Description, todo.Status)
    if todo.Completed && todo.Result != nil {
        fmt.Printf("  Result: %s\n", todo.Result.Output)
    }
    if todo.Error != nil {
        fmt.Printf("  Error: %v\n", todo.Error)
    }
}

// Count completed todos
completedCount := 0
for _, todo := range todos {
    if todo.Completed {
        completedCount++
    }
}
fmt.Printf("Progress: %d/%d completed\n", completedCount, len(todos))
```

## Examples

See the [plan mode example](../examples/plan_mode/) for a complete implementation demonstrating:
- Multi-step research workflow
- Progress tracking with visual indicators
- Custom tool integration
- Error handling and recovery

## Integration with MCP

Plan mode works seamlessly with MCP (Model Context Protocol) servers:

```go
// Connect to MCP server
mcpClient, err := mcp.NewStdioClient(ctx, "path/to/mcp-server")
if err != nil {
    panic(err)
}

// Create agent with MCP tools
agent := gollem.New(client, gollem.WithTools(mcpClient))

// Create plan that can use MCP tools
plan, err := agent.Plan(ctx, "Task that requires MCP tools")
```

This allows plan mode to leverage external tool ecosystems while maintaining the same planning and execution framework.

## Thread Safety Considerations

**Important**: Agent instances are not thread-safe. Follow these guidelines:

- **Single Goroutine**: Use one Agent instance per goroutine
- **External Synchronization**: If sharing an Agent across goroutines, implement proper mutex locking
- **Session State**: Session state is managed internally by the Agent and should not be accessed directly
- **Plan Execution**: Each Plan execution updates the Agent's session state

```go
// Safe: One agent per goroutine
go func() {
    agent := gollem.New(llmClient, options...)
    plan, _ := agent.Plan(ctx, "task")
    plan.Execute(ctx)
}()

// Unsafe: Concurrent access without synchronization
var agent = gollem.New(llmClient, options...)
go plan1.Execute(ctx) // May cause race conditions
go plan2.Execute(ctx) // May cause race conditions
```

## Data Access Design

Plan mode follows a strict external reference approach to avoid API confusion:

- **No Count Methods**: Instead of providing individual count methods (like `CountCompleted()`), use `GetToDos()` and process the returned data
- **Data Copying**: The `GetToDos()` method returns copies of internal data structures, preventing external modification
- **Immutable References**: All returned structures are designed to be read-only from the external perspective
- **Consistent Interface**: A single method provides access to all todo information, allowing flexible client-side processing
- **Session Access**: Use `plan.Session()` to access the plan's session and history; sessions are immutable once set
- **Single Source of Truth**: The plan's session is the authoritative source for conversation history, not internal fields

### Example Usage

```go
// Get all plan data
todos := plan.GetToDos()

// Process data client-side
pendingCount := 0
completedCount := 0
failedCount := 0

for _, todo := range todos {
    switch todo.Status {
    case "Pending":
        pendingCount++
    case "Completed":
        completedCount++
    case "Failed":
        failedCount++
    }
}

fmt.Printf("Plan Progress: %d completed, %d pending, %d failed\n",
    completedCount, pendingCount, failedCount)
```