package gollem

import "errors"

var (
	// ErrInvalidTool is returned when the tool validation of definition fails.
	ErrInvalidTool = errors.New("invalid tool specification")

	// ErrInvalidParameter is returned when the parameter validation of definition fails.
	ErrInvalidParameter = errors.New("invalid parameter")

	// ErrToolNameConflict is returned when the tool name is already used.
	ErrToolNameConflict = errors.New("tool name conflict")

	// ErrLoopLimitExceeded is returned when the session loop limit is exceeded. You can resume the session by calling the Prompt() method again.
	ErrLoopLimitExceeded = errors.New("loop limit exceeded")

	// ErrInvalidInputSchema is returned when the input schema from MCP is invalid or unsupported.
	ErrInvalidInputSchema = errors.New("invalid input schema")

	// ErrInvalidHistoryData is returned when the history data is invalid or unsupported.
	ErrInvalidHistoryData = errors.New("invalid history data")

	// ErrLLMTypeMismatch is returned when the LLM type is invalid or unsupported when loading history.
	ErrLLMTypeMismatch = errors.New("llm type mismatch")

	// ErrHistoryVersionMismatch is returned when the history version is invalid or unsupported.
	ErrHistoryVersionMismatch = errors.New("history version mismatch")

	// ErrExitConversation is returned when a tool signals that the conversation should be exited.
	// This error is treated as a successful completion of the conversation loop.
	ErrExitConversation = errors.New("exit conversation")
)
