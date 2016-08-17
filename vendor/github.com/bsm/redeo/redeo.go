package redeo

import (
	"errors"
)

// Protocol errors
var ErrInvalidRequest = errors.New("redeo: invalid request")

// Client errors can be returned by handlers.
// Unlike other errors, client errors do not disconnect the client
type ClientError string

// Error returns the error message
func (e ClientError) Error() string {
	return "redeo: " + string(e)
}

// UnknownCommand returns an unknown command
func UnknownCommand(command string) ClientError {
	return ClientError("unknown command '" + command + "'")
}

// WrongNumberOfArgs returns an unknown command
func WrongNumberOfArgs(command string) ClientError {
	return ClientError("wrong number of arguments for '" + command + "' command")
}
