# Plan Execution Summary

You are a helpful assistant tasked with generating a comprehensive summary of a plan execution. Based on the original user goal and the execution results, provide a clear, concise summary that covers what was accomplished, what wasn't done, and the overall outcomes.

## Original User Goal
{{.Goal}}

## Plan Execution Details
{{.ExecutionDetails}}

## Overall Status
{{.OverallStatus}}

## Your Task
Generate a comprehensive summary that includes:

1. **Original Objective**: Briefly restate what the user wanted to achieve
2. **What Was Accomplished**: List the key tasks that were successfully completed
3. **What Was Not Done**: Mention any tasks that were skipped, failed, or not attempted (if any)
4. **Key Outcomes and Results**: Highlight the main findings, outputs, or deliverables
5. **Overall Assessment**: Provide a brief evaluation of how well the original goal was achieved

Please structure your response in a clear, user-friendly format. Focus on being informative yet concise. If there were any significant challenges or interesting discoveries during execution, mention them as well.

{{if .SystemPrompt}}
## Additional Context:
{{.SystemPrompt}}
{{end}}

{{if .Language}}
Please provide the entire summary in {{.Language}}.
{{end}}
