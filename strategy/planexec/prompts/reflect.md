# Task Reflection

You have just completed a task. Review the progress and determine if any tasks need to be updated or added.

## Context

### Overall Goal
{{.Goal}}

### Completed Tasks
{{.CompletedTasks}}

### Remaining Tasks
{{.RemainingTasks}}

### Latest Task Result
{{.LatestResult}}

## Available Tools

{{.ToolList}}

## Understanding Tasks

**IMPORTANT**: Each task represents a **function/tool call execution**.

A task should specify:
- Which tool/function to call
- What parameters to pass
- What result is expected

## Evaluation Criteria

Based on the progress so far, determine:

1. **Task Retry/Modification**: Do any completed tasks (tool executions) need to be retried or modified?
   - Example: A tool call failed and needs different parameters or a different tool

2. **New Tasks**: Are there any NEW tool/function calls needed to achieve the goal?

## Important Guidelines

- **Only add new tasks if they require ESSENTIAL tool/function calls** not already covered by existing tasks
- Each new task must clearly specify which tool/function to execute
- If the remaining tasks are sufficient to achieve the goal, return empty arrays

## Response Format

Respond in JSON format:

```json
{
  "new_tasks": [
    "Call tool_name with parameter X to obtain Y",
    "Execute function_name to process Z"
  ],
  "updated_tasks": [
    {
      "id": "task-id",
      "description": "Retry tool_name with different parameter A",
      "state": "pending"
    }
  ],
  "reason": "Brief explanation of why these updates are needed"
}
```

### Field Descriptions

- `new_tasks`: Array of tool/function call descriptions. Add only if essential new tool executions are needed.
  - Each entry must specify which tool/function to call and with what parameters
- `updated_tasks`: Array of task modifications for tool executions that need to be retried or changed
  - Specify the task ID, new tool/function call description, and state
- `reason`: Brief explanation of your decision

### Example (No Updates Needed)

```json
{
  "new_tasks": [],
  "updated_tasks": [],
  "reason": "All tasks on track"
}
```
