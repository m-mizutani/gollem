# History Management

History represents a conversation history that can be used across different LLM sessions. It stores messages in a format specific to each LLM type (OpenAI, Claude, or Gemini).

## Automatic vs Manual History Management

### Automatic History Management (Recommended)

The `Execute` method provides automatic session management, eliminating the need for manual history handling:

```go
agent := gollem.New(client, gollem.WithTools(tools...))

// First interaction - creates new session automatically
err := agent.Execute(ctx, "Hello, I'm working on a project.")

// Follow-up - automatically remembers previous context
err = agent.Execute(ctx, "Can you help me with the next step?")

// Access conversation history if needed
history := agent.Session().History()
messageCount := history.ToCount()
```

**Benefits:**
- No manual history management required
- Conversation context preserved automatically
- Simplified API for conversational applications
- Reduced boilerplate code

### Manual History Management (Legacy)

For backward compatibility and advanced use cases, manual history management is still supported:

```go
// Legacy approach using Prompt method
var history *gollem.History

newHistory, err := agent.Prompt(ctx, "Hello", gollem.WithHistory(history))
if err != nil {
    return err
}
history = newHistory

// Continue conversation with manual history
newHistory, err = agent.Prompt(ctx, "Continue", gollem.WithHistory(history))
```

## Version Management

History includes version information to ensure compatibility. The current version is **3**.

### Version History

| Version | Changes |
|---------|---------|
| 1 | Initial format with provider-specific message dialects |
| 2 | Introduced unified message format across providers |
| 3 | Removed legacy function call fields and provider-specific dialects. Messages use a single canonical representation (`Message` with `MessageContent` typed data) |

### Compatibility

- **v1/v2 → v3 migration is not supported.** History serialized with v1 or v2 cannot be deserialized into v3; the format is a breaking change.
- If you have persisted v1/v2 histories, discard them or re-create the conversations with the current library version.
- Version is stored in the `"version"` JSON field of the serialized `History` struct. When deserializing, callers should verify that the version matches `gollem.HistoryVersion` before use.
- Future versions will document migration paths when feasible.

## Session Persistence

History is essential for maintaining conversation context across stateless sessions. Common use cases include:

### Backend Services
- **Stateless HTTP requests**: When your backend service receives requests from different instances or after restarts
- **Multiple API calls**: When you need to maintain conversation context across multiple API calls
- **Load balancing**: When sessions may be handled by different instances

### Distributed Systems
- **Microservices**: When conversations need to be shared across different services
- **Horizontal scaling**: When you need to load balance conversations across multiple servers
- **Service restarts**: When conversations need to be resumed after service restarts

### Long-running Conversations
- **Session resumption**: When conversations need to be resumed after service restarts
- **Conversation history**: When implementing features like "continue previous conversation"
- **Multi-session workflows**: When users switch between different devices or sessions

## Portability

History can be easily serialized/deserialized using standard JSON marshaling. This enables:

### Storage Options
- **Database persistence**: Store conversations in SQL or NoSQL databases
- **File storage**: Save conversations to local or cloud file systems
- **Cache systems**: Use Redis or Memcached for temporary storage
- **Message queues**: Transfer conversations through messaging systems

### Use Cases
- **Conversation backup**: Backup important conversations for disaster recovery
- **Analytics**: Analyze conversation patterns and user behavior
- **Audit trails**: Maintain records for compliance and debugging
- **Cross-platform sync**: Synchronize conversations across different platforms

## LLM Type Compatibility

Each History instance is tied to a specific LLM type (OpenAI, Claude, or Gemini). Important notes:

- Direct conversion between different LLM types is not supported
- Each LLM type has its own message format and capabilities
- History format is optimized for each LLM's specific requirements

## Usage Guidelines

### With Automatic Session Management (Recommended)

```go
// Create agent with automatic session management
agent := gollem.New(client,
    gollem.WithTools(tools...),
    gollem.WithSystemPrompt("You are a helpful assistant."),
)

// Execute multiple interactions - history managed automatically
err := agent.Execute(ctx, "What's the weather like?")
err = agent.Execute(ctx, "What about tomorrow?") // Remembers previous context

// Access history when needed
if history := agent.Session().History(); history != nil {
    messageCount := history.ToCount()
    fmt.Printf("Conversation has %d messages\n", messageCount)
    
    // Serialize for storage
    data, err := json.Marshal(history)
    if err != nil {
        return fmt.Errorf("failed to marshal history: %w", err)
    }
    
    // Store in database, file, etc.
    err = saveToDatabase(data)
}
```

### With Manual History Management (Legacy)

1. **Get History from Prompt response:**
   ```go
   // Create a new gollem agent
   agent := gollem.New(client)

   // Get response from Prompt
   history, err := agent.Prompt(ctx, "What is the weather?")
   if err != nil {
       return nil, fmt.Errorf("failed to get prompt response: %w", err)
   }
   ```

2. **Store the History for future use:**
   ```go
   // Store history in your database or storage
   jsonData, err := json.Marshal(history)
   if err != nil {
       return fmt.Errorf("failed to marshal history: %w", err)
   }
   
   // Save to your preferred storage
   err = database.SaveConversation(userID, jsonData)
   ```

3. **Use stored History in a new session:**
   ```go
   // Restore history
   jsonData, err := database.LoadConversation(userID)
   if err != nil {
       return fmt.Errorf("failed to load conversation: %w", err)
   }
   
   var restoredHistory gollem.History
   if err := json.Unmarshal(jsonData, &restoredHistory); err != nil {
       return fmt.Errorf("failed to unmarshal history: %w", err)
   }

   // Use history in next Prompt call
   newHistory, err := agent.Prompt(ctx, "What about tomorrow?", gollem.WithHistory(&restoredHistory))
   if err != nil {
       return nil, fmt.Errorf("failed to get prompt response: %w", err)
   }
   ```

Note: The History returned from Prompt contains the complete conversation history, so there's no need to manage or track individual messages. Each Prompt response provides a new History instance that includes all previous messages.

## Automatic History Persistence with HistoryRepository

`HistoryRepository` is an interface that lets gollem automatically load and save conversation history to any storage backend — filesystem, S3, GCS, a database, etc.

```go
type HistoryRepository interface {
    Load(ctx context.Context, sessionID string) (*History, error)
    Save(ctx context.Context, sessionID string, history *History) error
}
```

### How it works

- **On first `Execute`**: history is loaded from the repository using `sessionID`. If no history exists yet, the session starts fresh.
- **After each LLM round-trip**: history is saved automatically. This ensures that even if the process crashes mid-conversation, progress up to the last completed round-trip is preserved.
- `Load` returns `nil, nil` when the session ID is not found (new session — not an error).
- `Save` always overwrites the previous value for that session ID.

### Usage

```go
agent := gollem.New(client,
    gollem.WithHistoryRepository(repo, "user-123"),
)

// First run: loads history from repo (or starts fresh if none exists)
resp, err := agent.Execute(ctx, gollem.Text("Hello!"))

// Second run (same agent): session already exists, no Load is called again
resp, err = agent.Execute(ctx, gollem.Text("What did I just say?"))
```

> **Note**: `WithHistory` and `WithHistoryRepository` cannot be used together — an error is returned from `Execute` if both are set.

### Implementing HistoryRepository

The interface is intentionally minimal. A filesystem implementation looks like this (see also [examples/history](../examples/history/main.go)):

```go
type FileRepository struct{ dir string }

func (r *FileRepository) Load(ctx context.Context, id string) (*gollem.History, error) {
    data, err := os.ReadFile(filepath.Join(r.dir, id+".json"))
    if errors.Is(err, os.ErrNotExist) {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    var h gollem.History
    return &h, json.Unmarshal(data, &h)
}

func (r *FileRepository) Save(ctx context.Context, id string, h *gollem.History) error {
    data, _ := json.Marshal(h)
    return os.WriteFile(filepath.Join(r.dir, id+".json"), data, 0600)
}
```

For cloud storage, implement the same two methods using your SDK of choice — gollem imposes no additional constraints.

## Best Practices


## Next Steps

- Learn more about [tool creation](tools.md)
- Explore [MCP server integration](mcp.md)
- Check out [practical examples](examples.md)
- Review the [getting started guide](getting-started.md)
- Explore the [complete documentation](README.md)

