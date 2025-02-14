package core

type ErrorCode string

const (
	CodeValidationFailed  ErrorCode = "validation_failed"
	CodeNotFound          ErrorCode = "not_found"
	CodeInvalidTransition ErrorCode = "invalid_transition"
	CodeScheduleConflict  ErrorCode = "schedule_conflict"
	CodeInsufficientStock ErrorCode = "insufficient_stock"
)

// Error is a broken domain rule. Code is the value the API puts on the wire;
// Fields optionally maps a field name to a message for form-level errors.
type Error struct {
	Code    ErrorCode
	Message string
	Fields  map[string]string
}

func (e *Error) Error() string { return e.Message }

func Invalid(message string) *Error {
	return &Error{Code: CodeValidationFailed, Message: message}
}

func InvalidFields(message string, fields map[string]string) *Error {
	return &Error{Code: CodeValidationFailed, Message: message, Fields: fields}
}

func NotFound(message string) *Error {
	return &Error{Code: CodeNotFound, Message: message}
}

func InvalidTransition(message string) *Error {
	return &Error{Code: CodeInvalidTransition, Message: message}
}

func ScheduleConflict(message string) *Error {
	return &Error{Code: CodeScheduleConflict, Message: message}
}

func InsufficientStock(message string) *Error {
	return &Error{Code: CodeInsufficientStock, Message: message}
}
