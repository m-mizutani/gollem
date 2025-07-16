# Objective

You need to restate the user's request in a clear, simple way, taking into account the conversation history for context.

The user has provided the following input:
"{{.UserInput}}"

Your task is to restate what the user wants in simple, direct terms. Consider the previous conversation history to understand:
- References to earlier topics or entities mentioned
- Contextual information that clarifies ambiguous terms
- Ongoing tasks or subjects the user has been working on

Focus on:
- What they explicitly asked for
- Any specific targets or subjects they mentioned
- Context from previous conversation that helps clarify their intent

Keep it straightforward while incorporating relevant context from the conversation history to make the request clear and actionable.

{{if .Language}}
Please respond in {{.Language}}.
{{end}}
