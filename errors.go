package servantic

import "errors"

var (
	ErrInvalidTool      = errors.New("invalid tool specification")
	ErrInvalidParameter = errors.New("invalid parameter")

	ErrToolNameConflict  = errors.New("tool name conflict")
	ErrLoopLimitExceeded = errors.New("loop limit exceeded")

	// ErrInvalidInputSchema is returned when the input schema from MCPis invalid or unsupported.
	ErrInvalidInputSchema = errors.New("invalid input schema")
)
