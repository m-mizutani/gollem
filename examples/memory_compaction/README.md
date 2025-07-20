# Memory Compaction Example

This example demonstrates how to use gollem's history compaction feature to efficiently manage memory usage and token count during long conversations.

## Features

- **Automatic History Compaction**: Automatically compacts history when token thresholds are exceeded
- **In-Execute Compaction**: Checks for history compaction during Execute execution
- **Compaction Hook**: Monitor compaction events with detailed logging
- **Customizable Thresholds**: Configure when compaction should trigger

## How to Run

```bash
# Set OpenAI API key
export OPENAI_API_KEY="your-api-key-here"

# Run the example
go run main.go
```

## Configuration Options

### Compaction Settings

```go
// Create history compactor with custom options
compactor := gollem.NewHistoryCompactor(llmClient,
    gollem.WithMaxTokens(10000),           // Start compaction at 10k tokens
    gollem.WithPreserveRecentTokens(3000)) // Preserve 3k tokens of recent context

// Enable automatic compaction
agent := gollem.New(llmClient,
    gollem.WithHistoryCompactor(compactor),
    gollem.WithHistoryCompaction(true),
    gollem.WithCompactionHook(compactionHook))
```

### How Compaction Works

- **Summarization**: Old messages are summarized to preserve context while reducing token count
- **Recent Message Preservation**: Recent messages are kept intact to maintain conversation flow
- **Automatic Triggers**: Compaction occurs when token thresholds are exceeded

## Expected Output

```
=== History Compaction Demo ===
Compaction settings: MaxTokens=10000, PreserveRecentTokens=3000

--- Conversation 1 ---
User: Hello! My name is John. Nice to meet you today.
Assistant: Hello John! Nice to meet you too...
History status: 2 messages

--- Conversation 2 ---
User: I'm from New York and work as a programmer. I'm proficient in Go language.
Assistant: That's great! As a Go programmer...
History status: 4 messages

...

🗜️  History compaction executed: 12 → 6 messages (50.0% reduction)
📄 Summary: [Summary of older conversation]

--- Conversation 6 ---
User: Are there ways to reduce memory usage?
Assistant: Yes, there are several ways to reduce memory usage...
History status: 8 messages (compacted, original length: 12)
```

## Use Cases

- **Long Conversations**: Customer support or educational chatbots
- **Resource-Constrained Environments**: Mobile apps or edge devices
- **Cost Optimization**: Applications that need to minimize API usage costs
- **Complex Tool Chains**: Agents with numerous tool calls