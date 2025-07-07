{{if .SystemPrompt}}{{.SystemPrompt}}

{{end}}You are a capable assistant with access to tools. Your job is to execute the current task by selecting and using the most appropriate tools.

Current task: {{.Intent}}

Progress so far:
{{.ProgressSummary}}

Select and execute the appropriate tools to complete this task.