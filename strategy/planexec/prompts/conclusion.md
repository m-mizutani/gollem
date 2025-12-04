# Final Conclusion

All tasks have been completed. Based on the results, please provide a comprehensive response.

{{if .UserQuestion}}
## User's Original Question
{{.UserQuestion}}
{{end}}

{{if .UserIntent}}
## What the User Wants
{{.UserIntent}}

**THIS IS YOUR PRIMARY OBJECTIVE** - Address this intent clearly and naturally.
{{end}}

## Goal
{{.Goal}}

## Completed Tasks with Results
{{.CompletedTasks}}

## Instructions

{{if .UserIntent}}
**CRITICAL INSTRUCTIONS**:
1. **FIRST**: Address what the user wants to know or accomplish (the User Intent)
   - Present the key findings, results, or outcomes clearly
   - If it's a question, provide the information they need
   - If it's a task, summarize what was accomplished
   - If it's an analysis, present the discoveries and insights
2. **THEN**: Provide supporting details and evidence from the task results
3. Focus on **FINDINGS and RESULTS** (what was discovered or accomplished), not the process (what you did)
4. Do **NOT** say things like "I completed the tasks" or "I investigated" - present the findings naturally
5. Synthesize information across all tasks - don't just list them

Present your response now:
{{else}}
**IMPORTANT**:
1. Present the key findings, results, or outcomes from the completed tasks
2. Provide supporting details and context as needed
3. Focus on **FINDINGS and RESULTS** (what was discovered or accomplished), not on the process (what tasks were done)
4. Synthesize information across all tasks - don't just list them
{{end}}
