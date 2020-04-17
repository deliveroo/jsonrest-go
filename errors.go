package jsonrest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Error creates an error that will be rendered directly to the client.
func Error(status int, code, message string) *HTTPError {
	return &HTTPError{
		Status:  status,
		Code:    code,
		Message: message,
	}
}

// BadRequest returns an HTTP 400 Bad Request error with a custom error message.
func BadRequest(msg string) *HTTPError {
	return Error(http.StatusBadRequest, "bad_request", msg)
}

// NotFound returns an HTTP 404 Not Found error with a custom error message.
func NotFound(msg string) *HTTPError {
	return Error(http.StatusNotFound, "not_found", msg)
}

// Unauthorized returns an HTTP 401 Unauthorized error with a custom error
// message.
func Unauthorized(msg string) *HTTPError {
	return Error(http.StatusUnauthorized, "unauthorized", msg)
}

// UnprocessableEntity returns an HTTP 422 UnprocessableEntity error with a
// custom error message.
func UnprocessableEntity(msg string) *HTTPError {
	return Error(http.StatusUnprocessableEntity, "unprocessable_entity", msg)
}

// unknownError is returned for an internal server error.
var unknownError = &HTTPError{
	Code:    "unknown_error",
	Message: "an unknown error occurred",
	Status:  500,
}

// HTTPError is an error that will be rendered to the client.
type HTTPError struct {
	Code    string
	Message string
	Details []string
	Status  int

	wrapped error
}

// MarshalJSON implements the json.Marshaler interface.
func (err *HTTPError) MarshalJSON() ([]byte, error) {
	var wp struct {
		Error struct {
			Code    string   `json:"code"`
			Message string   `json:"message"`
			Details []string `json:"details,omitempty"`
		} `json:"error"`
	}
	wp.Error.Code = err.Code
	wp.Error.Message = err.Message
	wp.Error.Details = err.Details
	return json.Marshal(wp)
}

// Error implements the error interface.
func (err *HTTPError) Error() string {
	return fmt.Sprintf("jsonrest: %v: %v", err.Code, err.Message)
}

// Wrap wraps an inner error with the HTTPError.
func (err *HTTPError) Wrap(inner error) *HTTPError {
	err.wrapped = inner
	return err
}

// Unwrap returns the wrapped error, if any.
func (err *HTTPError) Unwrap() error {
	return err.wrapped
}

// Cause returns the wrapped error, if any. For compatibility with
// github.com/pkg/errors.
func (err *HTTPError) Cause() error {
	return err.wrapped
}

// translateError coerces err into an HTTPError that can be marshaled directly
// to the client.
func translateError(err error, dumpInternalError bool) *HTTPError {
	httpErr, ok := err.(*HTTPError)
	if !ok {
		httpErr = unknownError
		if dumpInternalError {
			httpErr.Details = dumpError(err)
		}
	}
	return httpErr
}

// dumpError formats the error suitable for viewing in a JSON response for local
// debugging.
func dumpError(err error) []string {
	s := fmt.Sprintf("%+v", err)           // stringify
	s = strings.Replace(s, "\t", "  ", -1) // tabs to spaces
	return strings.Split(s, "\n")          // split on newline
}
