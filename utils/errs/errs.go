package errs

import "fmt"

// Error type is used to create constant errors.
type Error string

func (e Error) Error() string { return string(e) }

func Errorf(format string, v ...interface{}) Error {
	return Error(fmt.Sprintf(format, v...))
}

// UnexpectedHTTPResponse is error interface implementation for HTTP response.
type UnexpectedHTTPResponse struct {
	StatusCode int
	Status     string
}

func (uhr UnexpectedHTTPResponse) Error() string {
	return fmt.Sprintf("UnexpectedHTTPResponse(%d '%s')", uhr.StatusCode, uhr.Status)
}
