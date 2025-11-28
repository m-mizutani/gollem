# Task Reflection

You have just completed a task. Review the progress and determine if the goal can be achieved with remaining tasks, or if updates are needed.

## Progress Tracking

**Current Iteration**: {{.CurrentIteration}} of {{.MaxIterations}}
**Completed Tasks**: {{.CompletedTaskCount}}
**Remaining Budget**: {{.RemainingIterations}} iterations

## Reflection Philosophy

Maximum results with minimum effort.

Before adding tasks, ask: can I answer the goal right now? If yes, you're done. If no, what single piece of information would make it possible?

Default to finishing. Adding tasks is expensive - only do it when absolutely necessary.

## Context

### User Intent
{{.UserIntent}}

This is what the user wants to know. All tasks should contribute to answering this intent.

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

## What You Know

This reflection has no access to the original system prompt. Use only:
- User Intent (what the user wants to know - THE PRIMARY OBJECTIVE)
- Overall Goal (what needs to be accomplished)
- Context Summary (background information from planning)
- Constraints (requirements from planning)
- Completed and remaining tasks
- Latest task result

## How to Reflect

Ask yourself these questions in order:

1. **Can I answer the user's intent with current information?**
   - Do I have the information the user wants to know?
   - If yes, you're done - mark remaining tasks as skipped
   - If no, continue to next question

2. **Are remaining tasks sufficient to fulfill the user's intent?**
   - Will they provide the information the user wants to know?
   - If yes, you're done - no updates needed
   - If no, continue to next question

3. **Did any pending tasks already execute?**
   - Check conversation history for tool calls
   - Mark duplicates as skipped

4. **Did the latest task fail or violate constraints?**
   - If yes, update it to retry with corrections
   - If no, continue to next question

5. **Is there one specific missing piece preventing us from fulfilling the user's intent?**
   - What information do we still need to answer what the user wants to know?
   - Add only that specific task
   - Be concrete about what tool to call and why

If you reach this point without updates, the remaining tasks are sufficient to fulfill the user's intent.

## What Makes a Good Update

Good updates are minimal and focused on the user's intent:
- Skip tasks that don't help answer what the user wants to know
- Skip tasks that are redundant or unnecessary
- Retry tasks that failed with specific corrections
- Add missing tasks only when you can't fulfill the user's intent without them

Bad updates expand scope beyond the user's intent:
- Exploring related topics not asked about
- Improving quality beyond what the user wants to know
- Adding "nice to have" information not requested
- Checking edge cases not mentioned in the user's intent

## Response Format

Respond in valid JSON only.

### No updates needed:
```json
{
  "new_tasks": [],
  "updated_tasks": [],
  "reason": "Remaining tasks sufficient to complete goal"
}
```

### With updates:
```json
{
  "new_tasks": [
    "Call specific_tool with parameter X to get missing information Y"
  ],
  "updated_tasks": [
    {
      "id": "task-123",
      "description": "Updated description if needed",
      "state": "skipped"
    }
  ],
  "reason": "Brief explanation"
}
```

Fields:
- `new_tasks`: Tool calls needed to complete the goal (empty if none needed)
- `updated_tasks`: Changes to existing tasks (empty if none needed)
  - Valid states: "pending", "in_progress", "completed", "skipped"
- `reason`: Why these updates are necessary
