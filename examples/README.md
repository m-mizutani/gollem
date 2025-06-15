# Gollem Examples

This directory contains various examples demonstrating the usage of Gollem with the new `Execute` method for automatic session management.

## 🚀 Basic Example
**[Basic Example](basic/main.go)** - A comprehensive example showing agent creation with custom tools, MCP integration, and automatic conversation history management.

**Features:**
- Custom tool integration
- MCP server integration (local and remote)
- Automatic session management with `Execute` method
- Interactive conversation loop
- Error handling

## 🌤️ Chat Example
**[Chat Example](chat/main.go)** - An interactive weather chat assistant demonstrating streaming responses and tool usage.

**Features:**
- Streaming response mode
- Weather tool with realistic data
- Automatic conversation history
- User-friendly interface
- Tool execution hooks

## 🔧 MCP Example
**[MCP Example](mcp/main.go)** - Demonstrates integration with Model Context Protocol servers for external tool access.

**Features:**
- Multiple MCP server connections (SSE and Stdio)
- Tool request monitoring
- File operations and external tools
- Comprehensive error handling

## 🔢 Tools Example
**[Tools Example](tools/main.go)** - Shows how to create and use custom mathematical tools with proper type definitions.

**Features:**
- Custom tool implementation
- Mathematical operations (Add, Multiply)
- Tool execution logging
- Type-safe parameter definitions

## 🎯 Simple Example
**[Simple Example](simple/main.go)** - A minimal example for quick testing with MCP tools.

**Features:**
- Single interaction example
- MCP tool integration
- Basic error handling
- Clean and simple code

## 📊 Embedding Example
**[Embedding Example](embedding/main.go)** - Demonstrates text embedding generation using OpenAI.

**Features:**
- Vector embedding generation
- Multiple text processing
- Dimension specification

## 🔄 Query Example
**[Query Example](query/main.go)** - Shows direct LLM querying without agent features.

**Features:**
- Direct session usage
- Multiple LLM provider support
- Simple text generation

## Key Improvements

All examples have been updated to use the new `Execute` method which provides:

- **Automatic Session Management**: No need to manually handle conversation history
- **Simplified API**: Just call `Execute` repeatedly for ongoing conversations
- **Better Error Handling**: Consistent error handling patterns
- **Enhanced User Experience**: More intuitive and user-friendly interfaces

## Migration from Old Examples

The examples have been migrated from the deprecated `Prompt` method to the new `Execute` method:

```go
// Old approach (deprecated)
history, err := agent.Prompt(ctx, "Hello", gollem.WithHistory(previousHistory))

// New approach (recommended)
err := agent.Execute(ctx, "Hello") // History managed automatically
```

## Running the Examples

Each example can be run independently. Make sure to set the required environment variables:

- `OPENAI_API_KEY` for OpenAI examples
- `GEMINI_PROJECT_ID` and `GEMINI_LOCATION` for Gemini examples
- `ANTHROPIC_API_KEY` for Claude examples

```bash
# Run the basic example
cd basic && go run main.go

# Run the chat example
cd chat && go run main.go

# Run other examples similarly
```
