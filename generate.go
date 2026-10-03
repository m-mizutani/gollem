package gollem

// GenerateOption configures a single Generate/Stream call.
// Options override session-level defaults for that call only.
//
// Example:
//
//	resp, err := session.Generate(ctx, inputs,
//	    gollem.WithTemperature(0.2),
//	    gollem.WithMaxTokens(256),
//	)
type GenerateOption func(*generateConfig)

// generateConfig holds per-call overrides for generation parameters.
// nil fields mean "use session default".
type generateConfig struct {
	responseSchema *Parameter
	temperature    *float64
	topP           *float64
	maxTokens      *int

	toolCallsDisabled bool
}

// NewGenerateConfig creates a generateConfig from the given options.
// This is required for LLM client implementations.
func NewGenerateConfig(opts ...GenerateOption) generateConfig { //nolint:revive
	cfg := generateConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// ResponseSchema returns the per-call response schema override, or nil if not set.
func (c *generateConfig) ResponseSchema() *Parameter {
	return c.responseSchema
}

// Temperature returns the per-call temperature override, or nil if not set.
func (c *generateConfig) Temperature() *float64 {
	return c.temperature
}

// TopP returns the per-call top-p override, or nil if not set.
func (c *generateConfig) TopP() *float64 {
	return c.topP
}

// MaxTokens returns the per-call max tokens override, or nil if not set.
func (c *generateConfig) MaxTokens() *int {
	return c.maxTokens
}

// ToolCallsDisabled reports whether tool calls are disabled for this call.
func (c *generateConfig) ToolCallsDisabled() bool {
	return c.toolCallsDisabled
}

// WithToolCallsDisabled forbids the model from calling tools in a single
// Generate/Stream call while the session's tool definitions are still sent.
//
// Removing the tools instead would change the request prefix. Claude binds
// thinking blocks to the system prompt and tool list they were produced with,
// so a history containing them is rejected when either changes; keeping the
// tools and forbidding their use leaves the prefix intact.
//
// Providers receive it as tool_choice "none" (Claude, OpenAI) or the function
// calling mode NONE (Gemini). Ollama has no equivalent parameter, so the
// Ollama client omits the tool definitions for that call instead. It has no
// effect when the session has no tools.
func WithToolCallsDisabled() GenerateOption {
	return func(cfg *generateConfig) {
		cfg.toolCallsDisabled = true
	}
}

// WithGenerateResponseSchema sets the response schema for a single Generate/Stream call.
func WithGenerateResponseSchema(schema *Parameter) GenerateOption {
	return func(cfg *generateConfig) {
		cfg.responseSchema = schema
	}
}

// WithTemperature sets the temperature for a single Generate/Stream call.
func WithTemperature(t float64) GenerateOption {
	return func(cfg *generateConfig) {
		cfg.temperature = &t
	}
}

// WithTopP sets the top-p for a single Generate/Stream call.
func WithTopP(p float64) GenerateOption {
	return func(cfg *generateConfig) {
		cfg.topP = &p
	}
}

// WithMaxTokens sets the max tokens for a single Generate/Stream call.
func WithMaxTokens(n int) GenerateOption {
	return func(cfg *generateConfig) {
		cfg.maxTokens = &n
	}
}
