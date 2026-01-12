package errs

type AppError struct {
	Code    string
	Message string
	Details interface{}
}

func (e AppError) Error() string {
	return e.Message
}

func New(code, message string, details interface{}) error {
	return AppError{Code: code, Message: message, Details: details}
}

