package redeo

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

// Request contains a command, arguments, and client information
type Request struct {
	Name string      `json:"name"`
	Args []string    `json:"args,omitempty"`
	Ctx  interface{} `json:"ctx,omitempty"`

	client *Client
}

// Client returns the client
func (r *Request) Client() *Client {
	return r.client
}

// WrongNumberOfArgs generates a standard client error
func (r *Request) WrongNumberOfArgs() ClientError {
	return WrongNumberOfArgs(r.Name)
}

// UnknownCommand generates a standard client error
func (r *Request) UnknownCommand() ClientError {
	return UnknownCommand(r.Name)
}

// ParseRequest parses a new request from a buffered connection
func ParseRequest(rd *bufio.Reader) (*Request, error) {
	line, err := rd.ReadString('\n')
	if err != nil {
		return nil, io.EOF
	} else if len(line) < 3 {
		return nil, io.EOF
	}

	// Truncate CRLF
	line = line[:len(line)-2]

	// Return if inline
	if line[0] != CodeBulkLen {
		return &Request{Name: strings.ToLower(line)}, nil
	}

	argc, err := strconv.Atoi(line[1:])
	if err != nil {
		return nil, ErrInvalidRequest
	}

	args := make([]string, argc)
	for i := 0; i < argc; i++ {
		if args[i], err = parseArgument(rd); err != nil {
			return nil, err
		}
	}
	return &Request{Name: strings.ToLower(args[0]), Args: args[1:]}, nil
}

func parseArgument(rd *bufio.Reader) (string, error) {
	line, err := rd.ReadString('\n')
	if err != nil {
		return "", io.EOF
	} else if len(line) < 3 {
		return "", io.EOF
	} else if line[0] != CodeStrLen {
		return "", ErrInvalidRequest
	}

	blen, err := strconv.Atoi(line[1 : len(line)-2])
	if err != nil {
		return "", ErrInvalidRequest
	}

	buf := make([]byte, blen+2)
	if _, err := io.ReadFull(rd, buf); err != nil {
		return "", io.EOF
	}

	return string(buf[:blen]), nil
}
