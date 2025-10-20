# Task Reflection

You have just completed a task. Review the progress and determine if any tasks need to be updated or added.

## Progress Tracking

**Current Iteration**: {{.CurrentIteration}} of {{.MaxIterations}}
**Completed Tasks**: {{.CompletedTaskCount}}
**Remaining Budget**: {{.RemainingIterations}} iterations

**CRITICAL**: You have LIMITED iterations remaining. The plan MUST complete within {{.RemainingIterations}} more iterations.
- Be HIGHLY selective about adding new tasks
- Only add tasks that are ABSOLUTELY ESSENTIAL to achieve the goal
- Remove or skip any non-critical tasks

## Context

### Overall Goal
{{.Goal}}

{{if .ContextSummary}}
### Context Summary
{{.ContextSummary}}
{{end}}

{{if .Constraints}}
### Constraints and Requirements
{{.Constraints}}
{{end}}

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

**CRITICAL**: This reflection has NO ACCESS to the original system prompt or conversation history. You MUST evaluate using ONLY the information provided above:
- **Overall Goal** - what needs to be accomplished
- **Context Summary** (if provided) - background information embedded during planning
- **Constraints and Requirements** (if provided) - compliance, security, quality requirements embedded during planning

Based on the progress so far, determine:

1. **Already Executed Tasks**: Have any pending tasks already been executed?
   - **Check the conversation history** to see which tools have already been called
   - If a pending task's tool/function was already called, mark the task as "skipped"
   - Example: If "Task 2: Call otx_file_hash" is pending but otx_file_hash was already called in the history, update Task 2 to state: "skipped"

2. **Unnecessary Tasks**: Are any remaining tasks no longer needed?
   - Review goal achievement status based on completed tasks
   - If the goal is already achieved, mark remaining tasks as "skipped"
   - If a task is no longer relevant due to other completed tasks, mark it as "skipped"

3. **Constraint Compliance**: Does the latest task result meet ALL constraints listed above?
   - Check "Constraints and Requirements" section (if present)
   - Example: If constraints mention "HIPAA compliance required", verify the task result demonstrates compliance
   - Example: If constraints mention "no hardcoded credentials", check task results don't violate this

4. **Goal Alignment**: Does the latest task result move toward the Overall Goal?
   - Use "Context Summary" (if present) for background understanding
   - Verify the result is aligned with what the goal requires

5. **Task Retry/Modification**: Do any completed tasks (tool executions) need to be retried or modified?
   - Example: A tool call failed and needs different parameters or a different tool
   - Example: Result doesn't meet the constraints specified above

6. **New Tasks**: Are there any NEW tool/function calls needed to achieve the goal?
   - Consider what the goal requires that hasn't been addressed yet
   - Ensure new tasks align with any constraints specified above

## Important Guidelines

- **Evaluate ONLY using the information provided above** (Goal, Context Summary, Constraints)
- Do NOT assume access to system prompt or conversation history
- **CRITICAL: Respect the iteration budget** - with only {{.RemainingIterations}} iterations left:
  - Do NOT add exploratory or investigative tasks
  - Only add tasks that are ABSOLUTELY ESSENTIAL to achieve the goal
  - Prioritize completing existing tasks over adding new ones
  - If uncertain whether a task is needed, DON'T add it
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
  - Valid states: "pending", "in_progress", "completed", "skipped"
  - Use "skipped" for tasks that are already executed or no longer needed
- `reason`: Brief explanation of your decision

### Example (No Updates Needed)

```json
{
  "new_tasks": [],
  "updated_tasks": [],
  "reason": "All tasks on track"
}
```
