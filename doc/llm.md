# LLM Provider Configuration

This document provides detailed configuration options for each LLM provider supported by gollem.

## Table of Contents

- [Gemini](#gemini)
- [Claude (Anthropic)](#claude-anthropic)
- [Claude (Vertex AI)](#claude-vertex-ai)
- [OpenAI](#openai)

## Gemini

### Basic Setup

```go
import (
    "context"
    "github.com/m-mizutani/gollem/llm/gemini"
)

client, err := gemini.New(ctx, "your-project-id", "us-central1")
```

### Authentication

Gemini uses Google Cloud credentials. Set up authentication using one of:

```bash
# Option 1: Service account key
export GOOGLE_APPLICATION_CREDENTIALS="path/to/service-account-key.json"

# Option 2: gcloud CLI
gcloud auth application-default login

# Option 3: Workload identity (automatic in GKE/Cloud Run)
```

### Configuration Options

#### Model Selection

```go
client, err := gemini.New(ctx, projectID, location,
    gemini.WithModel("gemini-1.5-pro-latest"),
)
```

Available models:
- `gemini-2.5-pro` - Most advanced model with state-of-the-art thinking capabilities
- `gemini-2.5-flash` - Best price-performance model with well-rounded capabilities
- `gemini-2.5-flash-lite` - Optimized for cost efficiency and low latency
- `gemini-2.0-flash` - Superior speed with native tool use and 1M token context
- `gemini-2.0-flash-thinking-exp-1219` - Experimental model with thinking capabilities

Note: Gemini 1.5 models are deprecated as of April 2025 for new projects.

#### Thinking Budget (Gemini 2.0)

Control the model's internal reasoning process:

```go
// Automatic thinking budget (model decides based on complexity)
client, err := gemini.New(ctx, projectID, location,
    gemini.WithThinkingBudget(-1),
)

// Fixed token budget for thinking
client, err := gemini.New(ctx, projectID, location,
    gemini.WithThinkingBudget(1000), // 1000 tokens
)

// Disable thinking
client, err := gemini.New(ctx, projectID, location,
    gemini.WithThinkingBudget(0),
)
```

The thinking budget controls computational effort for internal reasoning:
- **-1**: Automatic mode - the model decides based on task complexity
- **Positive value**: Fixed token budget for thinking
- **0**: Disable thinking mode

This feature is particularly useful for complex reasoning tasks where you want the model to spend more time thinking through problems before responding.

#### Temperature and Other Parameters

```go
client, err := gemini.New(ctx, projectID, location,
    gemini.WithTemperature(0.7),
    gemini.WithMaxTokens(2048),
    gemini.WithTopP(0.9),
)
```

### Environment Variables

- `GEMINI_PROJECT_ID` - Google Cloud project ID
- `GEMINI_LOCATION` - Vertex AI location (e.g., "us-central1")
- `GOLLEM_LOGGING_GEMINI_PROMPT` - Enable prompt logging for debugging
- `GOLLEM_LOGGING_GEMINI_RESPONSE` - Enable response logging for debugging

## Claude (Anthropic)

### Basic Setup

```go
import (
    "context"
    "github.com/m-mizutani/gollem/llm/claude"
)

client, err := claude.New(ctx, "your-api-key")
```

### Configuration Options

#### Model Selection

```go
client, err := claude.New(ctx, apiKey,
    claude.WithModel("claude-3-5-sonnet-20241022"),
)
```

Available models:
- `claude-opus-4-1-20250805` - Most powerful model, best for complex tasks (August 2025)
- `claude-sonnet-4-20250514` - Balanced performance and efficiency
- `claude-3-5-sonnet-20241022` - Previous generation, still widely available
- `claude-3-5-haiku-20241022` - Fast, cost-effective model

Note: Claude Opus 4.1 and Sonnet 4 are hybrid models offering both instant and extended thinking modes.

#### Temperature and Max Tokens

```go
client, err := claude.New(ctx, apiKey,
    claude.WithTemperature(0.7),
    claude.WithMaxTokens(4096),
)
```

### Environment Variables

- `ANTHROPIC_API_KEY` - Anthropic API key
- `GOLLEM_LOGGING_CLAUDE_PROMPT` - Enable prompt logging
- `GOLLEM_LOGGING_CLAUDE_RESPONSE` - Enable response logging

## Claude (Vertex AI)

### Basic Setup

```go
import (
    "context"
    "github.com/m-mizutani/gollem/llm/claude"
)

client, err := claude.NewWithVertex(ctx, "us-central1", "your-project-id")
```

### Configuration Options

#### Model Selection

```go
client, err := claude.NewWithVertex(ctx, region, projectID,
    claude.WithVertexModel("claude-sonnet-4@20250514"),
)
```

Available models on Vertex AI:
- `claude-opus-4-1@20250805` - Most powerful model (if available in your region)
- `claude-sonnet-4@20250514` - Latest Claude Sonnet model
- `claude-3-5-sonnet@20241022` - Previous generation Sonnet
- `claude-3-5-haiku@20241022` - Fast, cost-effective model

#### System Prompt

```go
client, err := claude.NewWithVertex(ctx, region, projectID,
    claude.WithVertexSystemPrompt("You are a helpful assistant."),
)
```

### Authentication

Uses Google Cloud credentials (same as Gemini):

```bash
# Option 1: Service account key
export GOOGLE_APPLICATION_CREDENTIALS="path/to/service-account-key.json"

# Option 2: gcloud CLI
gcloud auth application-default login
```

### Benefits of Vertex AI Integration

- Unified Google Cloud billing and cost management
- Enterprise security with VPC, private endpoints, and audit logs
- Regional deployment for data residency requirements
- Vertex AI MLOps integration for monitoring and management

## OpenAI

### Basic Setup

```go
import (
    "context"
    "github.com/m-mizutani/gollem/llm/openai"
)

client, err := openai.New(ctx, "your-api-key")
```

### Configuration Options

#### Model Selection

```go
client, err := openai.New(ctx, apiKey,
    openai.WithModel("gpt-4-turbo-preview"),
)
```

Available models:
- `o3-pro` - Most powerful reasoning model with extended thinking
- `o3` - Advanced reasoning model for complex tasks
- `o4-mini` - Fast, cost-efficient reasoning model
- `gpt-4.1` - Latest GPT model with 1M token context (June 2024 cutoff)
- `gpt-4.1-mini` - Smaller version of GPT-4.1
- `gpt-4o` - Previous generation, still available
- `gpt-4o-mini` - Smaller, faster GPT-4o variant
- `gpt-3.5-turbo` - Legacy model, cost-effective

Note: GPT-4.5 is in research preview. o1 models are being phased out in favor of o3/o4 series.

#### Temperature and Other Parameters

```go
client, err := openai.New(ctx, apiKey,
    openai.WithTemperature(0.7),
    openai.WithMaxTokens(2048),
    openai.WithTopP(0.9),
    openai.WithFrequencyPenalty(0.5),
    openai.WithPresencePenalty(0.5),
)
```

#### Organization and Base URL

```go
client, err := openai.New(ctx, apiKey,
    openai.WithOrganization("org-id"),
    openai.WithBaseURL("https://custom-endpoint.com"),
)
```

### Environment Variables

- `OPENAI_API_KEY` - OpenAI API key
- `OPENAI_ORGANIZATION` - Organization ID (optional)
- `GOLLEM_LOGGING_OPENAI_PROMPT` - Enable prompt logging
- `GOLLEM_LOGGING_OPENAI_RESPONSE` - Enable response logging

## Common Configuration Patterns

### Session Configuration

All LLM clients support common session options:

```go
session, err := client.NewSession(ctx,
    gollem.WithSessionHistory(history),
    gollem.WithSessionContentType(gollem.ContentTypeJSON),
    gollem.WithSessionTools(tool1, tool2),
    gollem.WithSessionSystemPrompt("You are a helpful assistant."),
)
```

### Embedding Generation

Providers that support embeddings (OpenAI and Gemini):

```go
embeddings, err := client.GenerateEmbedding(ctx, 
    768,           // dimension
    []string{      // texts to embed
        "Hello world",
        "Another text",
    },
)
```

### Error Handling

All providers return standardized errors that can be checked:

```go
resp, err := session.GenerateContent(ctx, input)
if err != nil {
    // Check for specific error types
    // Handle token limit errors, rate limits, etc.
    return err
}
```

## Debugging and Monitoring

### Enable Logging

Set environment variables to enable detailed logging:

```bash
# Enable all prompt logging
export GOLLEM_LOGGING_CLAUDE_PROMPT=true
export GOLLEM_LOGGING_OPENAI_PROMPT=true
export GOLLEM_LOGGING_GEMINI_PROMPT=true

# Enable all response logging
export GOLLEM_LOGGING_CLAUDE_RESPONSE=true
export GOLLEM_LOGGING_OPENAI_RESPONSE=true
export GOLLEM_LOGGING_GEMINI_RESPONSE=true
```

### Log Output Format

Logs are structured with ctxlog scopes:

```json
{
  "level": "info",
  "scope": "claude_response",
  "model": "claude-3-sonnet-20240229",
  "usage": {
    "input_tokens": 150,
    "output_tokens": 75
  }
}
```