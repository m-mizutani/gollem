# Final Conclusion

All tasks have been completed. Based on the results, please provide a comprehensive answer.

{{if .UserQuestion}}
## User's Question
{{.UserQuestion}}
{{end}}

## Goal
{{.Goal}}

## Completed Tasks with Results
{{.CompletedTasks}}

## Instructions

{{if .UserQuestion}}
**IMPORTANT**:
1. First, provide a **DIRECT answer** to the user's question (e.g., "Yes, found X" or "No, not found")
2. Then provide supporting details from the task results
3. Focus on **FINDINGS and RESULTS**, not on what tasks were executed
4. Do **NOT** just list completed tasks - synthesize the information to answer the question

Answer the user's question now:
{{else}}
**IMPORTANT**: Provide a clear conclusion or answer based on the task results. Focus on **FINDINGS and RESULTS** (what was discovered), not on the process (what tasks were done). Synthesize the information rather than listing tasks.
{{end}}
