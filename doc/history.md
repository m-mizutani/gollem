# History

History represents a conversation history that can be used across different LLM sessions. It stores messages in a format specific to each LLM type (GPT, Claude, or Gemini).

## Version Management

History includes version information to ensure compatibility:

- Current version: 1
- Version checking is performed when converting between formats
- Version mismatch will result in an error
- This helps maintain compatibility when the History structure changes in future updates

## Session Persistence

History is essential for maintaining conversation context across stateless sessions. Common use cases include:

- Backend services handling stateless HTTP requests
  - When your backend service receives requests from different instances or after restarts
  - When you need to maintain conversation context across multiple API calls
- Distributed systems
  - When sessions may be handled by different instances
  - When you need to load balance conversations across multiple servers
- Long-running conversations
  - When conversations need to be resumed after service restarts
  - When implementing features like "continue previous conversation"

## Portability

History can be easily serialized/deserialized using standard JSON marshaling. This enables:

- Storing conversations in databases
  - Persist conversations for future reference
  - Implement conversation history features
- Transferring conversations between services
  - Move conversations between different environments
  - Share conversations across microservices
- Implementing conversation backup and restore features
  - Backup important conversations
  - Restore conversations after system failures

## LLM Type Compatibility

Each History instance is tied to a specific LLM type (GPT, Claude, or Gemini). Important notes:

- Direct conversion between different LLM types is not supported
- Each LLM type has its own message format and capabilities

## Usage Guidelines

1. Get History from Prompt response:
   ```go
   // Create a new gollem instance
   g := gollem.New(llmClient)

   // Get response from Prompt
   history, err := g.Prompt(ctx, "What is the weather?")
   if err != nil {
       return nil, fmt.Errorf("failed to get prompt response: %w", err)
   }
   ```

2. Store the History for future use:
   ```go
   // Store history in your database or storage
   jsonData, err := json.Marshal(history)
   if err != nil {
       return fmt.Errorf("failed to marshal history: %w", err)
   }
   ```

3. Use stored History in a new session:
   ```go
   // Restore history
   var restoredHistory History
   if err := json.Unmarshal(jsonData, &restoredHistory); err != nil {
       return fmt.Errorf("failed to unmarshal history: %w", err)
   }

   // Use history in next Prompt call
   newHistory, err := g.Prompt(ctx, "What about tomorrow?", gollem.WithHistory(&restoredHistory))
   if err != nil {
       return nil, fmt.Errorf("failed to get prompt response: %w", err)
   }
   ```

Note: The History returned from Prompt contains the complete conversation history, so there's no need to manage or track individual messages. Each Prompt response provides a new History instance that includes all previous messages.

## Best Practices

1. **Error Handling**
   - Always check for errors when converting between formats
   - Handle type mismatches gracefully
   - Check for version compatibility
   - Implement proper error handling for version mismatches

2. **Storage Considerations**
   - Consider the size of your conversations
   - Implement cleanup strategies for old conversations

3. **Security**
   - Implement proper access controls

