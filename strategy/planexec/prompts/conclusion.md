# Final Conclusion

All tasks have been completed. Based on the results, please provide a comprehensive answer.

{{if .UserQuestion}}
## User's Original Question
{{.UserQuestion}}
{{end}}

{{if .UserIntent}}
## What the User Wants to Know
{{.UserIntent}}

**THIS IS YOUR PRIMARY OBJECTIVE** - Answer this intent directly.
{{end}}

## Goal
{{.Goal}}

## Completed Tasks with Results
{{.CompletedTasks}}

## Instructions

{{if .UserIntent}}
**CRITICAL INSTRUCTIONS**:
1. **FIRST**: Provide a **DIRECT answer** to what the user wants to know (the User Intent)
   - If they want to know "what you found", state what you found
   - If they want to know "if X exists", answer Yes/No with location/details
   - If they want to know "how X works", explain how it works
2. **THEN**: Provide supporting details and evidence from the task results
3. Focus on **FINDINGS and RESULTS** (what was discovered), not the process (what you did)
4. Do **NOT** say things like "I completed the tasks" or "I investigated" - just present the findings
5. Synthesize information across all tasks - don't just list them

Answer what the user wants to know now:
{{else}}
**IMPORTANT**: Provide a clear conclusion or answer based on the task results. Focus on **FINDINGS and RESULTS** (what was discovered), not on the process (what tasks were done). Synthesize the information rather than listing tasks.
{{end}}
