package internal

import "github.com/m-mizutani/ctxlog"

var (
	ScopeGollem = ctxlog.NewScope("gollem")
	ScopeLLM    = ScopeGollem.NewChild("llm", ctxlog.EnabledBy("GOLLEM_LOGGING_LLM"))
	
	// Provider-level scopes
	ScopeGemini = ScopeLLM.NewChild("gemini", ctxlog.EnabledBy("GOLLEM_LOGGING_GEMINI"))
	ScopeClaude = ScopeLLM.NewChild("claude", ctxlog.EnabledBy("GOLLEM_LOGGING_CLAUDE"))
	ScopeOpenAI = ScopeLLM.NewChild("openai", ctxlog.EnabledBy("GOLLEM_LOGGING_OPENAI"))
	
	// Granular scopes for debugging specific operations
	ScopeGeminiGenerate = ScopeGemini.NewChild("generate", ctxlog.EnabledBy("GOLLEM_LOGGING_GEMINI_GENERATE"))
	ScopeGeminiStream   = ScopeGemini.NewChild("stream", ctxlog.EnabledBy("GOLLEM_LOGGING_GEMINI_STREAM"))
	ScopeGeminiHistory  = ScopeGemini.NewChild("history", ctxlog.EnabledBy("GOLLEM_LOGGING_GEMINI_HISTORY"))
	
	ScopeClaudeGenerate = ScopeClaude.NewChild("generate", ctxlog.EnabledBy("GOLLEM_LOGGING_CLAUDE_GENERATE"))
	ScopeClaudeStream   = ScopeClaude.NewChild("stream", ctxlog.EnabledBy("GOLLEM_LOGGING_CLAUDE_STREAM"))
	ScopeClaudeConvert  = ScopeClaude.NewChild("convert", ctxlog.EnabledBy("GOLLEM_LOGGING_CLAUDE_CONVERT"))
	
	ScopeOpenAIGenerate = ScopeOpenAI.NewChild("generate", ctxlog.EnabledBy("GOLLEM_LOGGING_OPENAI_GENERATE"))
	ScopeOpenAIStream   = ScopeOpenAI.NewChild("stream", ctxlog.EnabledBy("GOLLEM_LOGGING_OPENAI_STREAM"))
	ScopeOpenAIConvert  = ScopeOpenAI.NewChild("convert", ctxlog.EnabledBy("GOLLEM_LOGGING_OPENAI_CONVERT"))
)
