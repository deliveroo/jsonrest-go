package jsonrest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// HTTPErrorResponse allows to customize the format of non-200 http responses.
// If the handler returns an error which implements this interface, it will
// be marshaled as-is to the response and written with the specified status
// code.
type HTTPErrorResponse interface {
	error
	StatusCode() int
}

// Error creates an error that will be rendered directly to the client.
func Error(status int, code, message string) *HTTPError {
	return &HTTPError{
		Status:  status,
		Code:    code,
		Message: message,
	}
}

// BadRequest returns an HTTP 400 Bad Request error with a custom error message.
func BadRequest(code, msg string) *HTTPError {
	return Error(http.StatusBadRequest, code, msg)
}

// NotFound returns an HTTP 404 Not Found error with a custom error message.
func NotFound(code, msg string) *HTTPError {
	return Error(http.StatusNotFound, code, msg)
}

// Unauthorized returns an HTTP 401 Unauthorized error with a custom error
// message.
func Unauthorized(code, msg string) *HTTPError {
	return Error(http.StatusUnauthorized, code, msg)
}

// UnprocessableEntity returns an HTTP 422 UnprocessableEntity error with a
// custom error message.
func UnprocessableEntity(code, msg string) *HTTPError {
	return Error(http.StatusUnprocessableEntity, code, msg)
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

// StatusCode implements the HTTPErrorResponse interface.
func (err *HTTPError) StatusCode() int {
	return err.Status
}

// MarshalJSON implements the json.Marshaler interface.
func (err *HTTPError) MarshalJSON() ([]byte, error) {
	var wp struct {
		Error struct {
			Status  string   `json:"status,omitempty"`
			Code    string   `json:"code"`
			Message string   `json:"message"`
			Details []string `json:"details,omitempty"`
		} `json:"error"`
	}
	wp.Error.Code = err.Code
	wp.Error.Message = err.Message
	wp.Error.Details = err.Details
	wp.Error.Status = strings.ReplaceAll(strings.ToLower(http.StatusText(err.Status)), " ", "_")
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

// translateError coerces err into an HTTPErrorResponse that can be marshaled directly
// to the client.
func translateError(err error, dumpInternalError bool) HTTPErrorResponse {
	errResponse, ok := err.(HTTPErrorResponse)
	if !ok {
		e := *unknownError
		httpErr := &(e) // shallow copy
		if dumpInternalError {
			httpErr.Details = dumpError(err)
		}
		errResponse = httpErr
	}
	return errResponse
}

// dumpError formats the error suitable for viewing in a JSON response for local
// debugging.
func dumpError(err error) []string {
	s := fmt.Sprintf("%+v", err)           // stringify
	s = strings.Replace(s, "\t", "  ", -1) // tabs to spaces
	return strings.Split(s, "\n")          // split on newline
}
