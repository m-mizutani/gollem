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
- **History**: Optional conversation history maintained across steps

### ToDo Structure

Each plan step (ToDo) contains:
- **Description**: Detailed description of what needs to be done
- **Intent**: High-level intention or goal of the step
- **Status**: Current status (pending, executing, completed, failed, skipped)
- **Result**: Execution results including tool calls and responses
- **Error**: Any error that occurred during execution

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

restoredPlan, err := agent.DeserializePlan(data)
if err != nil {
    panic(err)
}

// Continue execution
result, err := restoredPlan.Execute(context.Background())
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
    gollem.WithMaxPlanSteps(10),                    // Limit number of steps
    gollem.WithReflectionEnabled(true),             // Enable reflection
    gollem.WithPlanSystemPrompt("Custom prompt"),   // Custom system prompt
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
    gollem.WithToDoStartHook(func(ctx context.Context, todo gollem.PlanToDoPublic) error {
        fmt.Printf("Starting: %s\n", todo.Description)
        return nil
    }),
    gollem.WithToDoCompletedHook(func(ctx context.Context, todo gollem.PlanToDoPublic) error {
        fmt.Printf("Completed: %s (Status: %s)\n", todo.Description, todo.Status)
        return nil
    }),
)
```

## Advanced Features

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
        fmt.Println("Plan is missing required components")
    default:
        fmt.Printf("Plan execution failed: %v\n", err)
    }
}
```

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
    return gollem.WithToDoStartHook(func(ctx context.Context, todo gollem.PlanToDoPublic) error {
        fmt.Printf("🔄 %s\n", todo.Description)
        return nil
    })
}

func createCompletionMonitor() gollem.PlanOption {
    return gollem.WithToDoCompletedHook(func(ctx context.Context, todo gollem.PlanToDoPublic) error {
        status := "✅"
        if todo.Status == "failed" {
            status = "❌"
        } else if todo.Status == "skipped" {
            status = "⏭️"
        }
        fmt.Printf("%s %s\n", status, todo.Description)
        return nil
    })
}
```

### Error Recovery

Implement graceful error handling:

```go
plan, err := agent.Plan(context.Background(), "complex task",
    gollem.WithPlanStepCompletedHook(func(ctx context.Context, plan *gollem.Plan, todo gollem.PlanToDoInternal) error {
        if todo.Error != nil {
            // Log error but continue execution
            log.Printf("Step failed: %s - %v", todo.Description, todo.Error)
        }
        return nil
    }),
)
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