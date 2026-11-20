// Package core errors for agent execution.
package core

import "fmt"

// ExecutionError represents an error during agent execution.
type ExecutionError struct {
	AgentName string
	Code      string // "validation", "not_found", "internal", etc
	Message   string
	Cause     error
}

func (e *ExecutionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s (%s): %s: %v", e.AgentName, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s (%s): %s", e.AgentName, e.Code, e.Message)
}

// NewExecutionError creates an execution error.
func NewExecutionError(agentName, code, message string, cause error) *ExecutionError {
	return &ExecutionError{
		AgentName: agentName,
		Code:      code,
		Message:   message,
		Cause:     cause,
	}
}

// Common error codes.
const (
	ErrorCodeValidation = "validation"
	ErrorCodeNotFound   = "not_found"
	ErrorCodeInternal   = "internal"
	ErrorCodeTimeout    = "timeout"
	ErrorCodeCanceled   = "canceled"
)

// ValidationError is a validation-specific error.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}
