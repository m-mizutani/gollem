# SubAgent Middleware with Session Access Example

This example demonstrates how SubAgent middleware can access session history for post-execution processing.

## Features Demonstrated

### Pre-execution Processing
- Injecting context information (timestamps, user data, environment info)
- Modifying arguments before template rendering

### Post-execution Processing
- Accessing session history via `result.Session`
- Extracting metrics (message count, execution duration, token usage)
- Memory extraction from conversation history
- Modifying result data based on session analysis

## Prerequisites

Set your OpenAI API key:

```bash
export OPENAI_API_KEY="your-api-key-here"
```

## Running the Example

```bash
go run main.go
```

## Code Structure

The middleware function demonstrates both phases:

```go
func createMiddleware() func(gollem.SubAgentHandler) gollem.SubAgentHandler {
    return func(next gollem.SubAgentHandler) gollem.SubAgentHandler {
        return func(ctx context.Context, args map[string]any) (gollem.SubAgentResult, error) {
            // Pre-execution: Inject context
            args["_execution_time"] = time.Now().Format(time.RFC3339)
            args["_user_context"] = "Example user"

            // Execute
            result, err := next(ctx, args)
            if err != nil {
                return gollem.SubAgentResult{}, err
            }

            // Post-execution: Access session and extract insights
            history, _ := result.Session.History()
            if history != nil {
                result.Data["metrics"] = map[string]any{
                    "message_count": len(history.Messages),
                    "duration": executionDuration,
                }
            }

            return result, nil
        }
    }
}
```

## Use Cases

This pattern is particularly useful for:

1. **Memory-aware Agents**: Extract and save learned information from conversation history
2. **Record Extraction**: Parse structured data from session responses
3. **Metrics Collection**: Track token usage, execution time, and performance
4. **Context Injection**: Add dynamic context based on user, environment, or system state
5. **Audit Logging**: Record detailed execution traces for compliance

## Key Concepts

- **SubAgentResult**: Contains both `Data` (result for parent) and `Session` (for middleware)
- **Session Access**: Middleware can call `result.Session.History()` to access conversation
- **Read-only**: Session is provided for analysis, not modification
- **Error Handling**: Session access errors should be handled gracefully (log and continue)

## Related Documentation

- See `CHANGELOG.md` for migration guide from older versions
- See `doc/architecture.md` for SubAgent architecture details
