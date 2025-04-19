package servant

import "errors"

var (
	ErrInvalidTool      = errors.New("invalid tool specification")
	ErrInvalidParameter = errors.New("invalid parameter")
)
