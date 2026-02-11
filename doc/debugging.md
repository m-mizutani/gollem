# Debugging

## Logging LLM Requests and Responses

You can enable detailed logging for debugging purposes by setting environment variables.

### Prompt Logging

Log all prompts sent to each LLM provider:

| Environment Variable | Provider |
|---|---|
| `GOLLEM_LOGGING_CLAUDE_PROMPT=true` | Claude (Anthropic) |
| `GOLLEM_LOGGING_OPENAI_PROMPT=true` | OpenAI |
| `GOLLEM_LOGGING_GEMINI_PROMPT=true` | Gemini |

### Response Logging

Log all responses from each LLM provider:

| Environment Variable | Provider |
|---|---|
| `GOLLEM_LOGGING_CLAUDE_RESPONSE=true` | Claude (Anthropic) |
| `GOLLEM_LOGGING_OPENAI_RESPONSE=true` | OpenAI |
| `GOLLEM_LOGGING_GEMINI_RESPONSE=true` | Gemini |

### Usage

```bash
# Enable Claude prompt and response logging
export GOLLEM_LOGGING_CLAUDE_PROMPT=true
export GOLLEM_LOGGING_CLAUDE_RESPONSE=true

# Enable OpenAI response logging only
export GOLLEM_LOGGING_OPENAI_RESPONSE=true

# Enable all Gemini logging
export GOLLEM_LOGGING_GEMINI_PROMPT=true
export GOLLEM_LOGGING_GEMINI_RESPONSE=true
```

These will output the raw prompts and responses to the standard logger, which is useful for debugging conversation flow, tool usage, and token consumption.

### Log Output Format

Logs are structured with ctxlog scopes:

```json
{
  "level": "info",
  "scope": "claude_response",
  "model": "claude-3-sonnet-20240229",
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 150,
    "output_tokens": 75
  },
  "content": [
    {
      "type": "text",
      "text": "Generated response text"
    },
    {
      "type": "tool_use",
      "id": "call_123",
      "name": "search_function",
      "input": {"query": "example"}
    }
  ]
}
```

### Benefits

- **Debugging**: Track exact prompts and responses during development
- **Monitoring**: Observe token usage and response patterns
- **Audit**: Log tool calls and function executions
- **Performance**: Analyze response times and token efficiency
- **Troubleshooting**: Capture complete interaction context for issue resolution

## Next Steps

- Learn about [tracing](tracing.md) for structured execution observability
- Review [LLM provider configuration](llm.md) for provider-specific settings
