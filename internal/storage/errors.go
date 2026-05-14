package storage

import "errors"

var (
	ErrClosed           = errors.New("storage log is closed")
	ErrOffsetOutOfRange = errors.New("offset out of range")
	ErrRecordTooLarge   = errors.New("record too large")
	ErrInvalidRecord    = errors.New("invalid record")
)
