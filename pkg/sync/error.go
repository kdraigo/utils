package sync

import "sync/atomic"

// Error is an atomic type-safe wrapper for error values.
type Error struct {
	_ nocmp // disallow non-atomic comparison

	v Value
}

// NewError creates a new Error.
func NewError(val error) *Error {
	x := &Error{}
	if val != _zeroError {
		x.Store(val)
	}
	return x
}

// Load atomically loads the wrapped error.
func (x *Error) Load() error {
	return unpackError(x.v.Load())
}

// Store atomically stores the passed error.
func (x *Error) Store(val error) {
	x.v.Store(packError(val))
}

// CompareAndSwap is an atomic compare-and-swap for error values.
func (x *Error) CompareAndSwap(old, new error) (swapped bool) {
	if x.v.CompareAndSwap(packError(old), packError(new)) {
		return true
	}

	if old == _zeroError {
		// If the old value is the empty value, then it's possible the
		// underlying Value hasn't been set and is nil, so retry with nil.
		return x.v.CompareAndSwap(nil, packError(new))
	}

	return false
}

// Swap atomically stores the given error and returns the old
// value.
func (x *Error) Swap(val error) (old error) {
	return unpackError(x.v.Swap(packError(val)))
}

type packedError struct{ Value error }

func packError(v error) interface{} {
	return packedError{v}
}

func unpackError(v interface{}) error {
	if err, ok := v.(packedError); ok {
		return err.Value
	}
	return nil
}

type nocmp [0]func()

var _zeroError error

// Value for atomic handling of data.
type Value struct {
	_ nocmp // disallow non-atomic comparison

	atomic.Value
}
