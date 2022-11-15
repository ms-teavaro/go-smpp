package smpp

import "errors"

var (
	ErrConnectionClosed = errors.New("smpp: connection closed")
	ErrContextDone      = errors.New("smpp: context done")
)
