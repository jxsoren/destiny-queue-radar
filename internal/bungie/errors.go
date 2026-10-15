package bungie

import (
	"errors"
	"fmt"
)

// Bungie PlatformErrorCodes used by this app.
const (
	CodeSuccess         = 1
	CodeSystemDisabled  = 5
	CodeAccountNotFound = 1601
)

var ErrPlayerNotFound = errors.New("bungie: player not found")

// APIError is a non-success Bungie envelope, or an undecodable HTTP response.
type APIError struct {
	Code            int
	Status          string
	Message         string
	ThrottleSeconds int
	HTTPStatus      int
}

func (e *APIError) Error() string {
	if e.Code == 0 {
		return fmt.Sprintf("bungie: http %d: %s", e.HTTPStatus, e.Message)
	}
	return fmt.Sprintf("bungie: error %d %s: %s", e.Code, e.Status, e.Message)
}

// TransportError wraps network-level failures (DNS, connection reset, timeout).
type TransportError struct {
	Path string
	Err  error
}

func (e *TransportError) Error() string { return fmt.Sprintf("bungie: request %s: %v", e.Path, e.Err) }
func (e *TransportError) Unwrap() error { return e.Err }

func IsSystemDisabled(err error) bool { return hasCode(err, CodeSystemDisabled) }

func IsNotFound(err error) bool {
	return errors.Is(err, ErrPlayerNotFound) || hasCode(err, CodeAccountNotFound)
}

func IsThrottled(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.ThrottleSeconds > 0
}

func hasCode(err error, code int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Code == code
}
