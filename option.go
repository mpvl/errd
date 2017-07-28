// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errd

import "os"

// An Option configures error and defer handling.
type Option option

type option func(c *Config)

// A Handler processes errors.
type Handler interface {
	Handle(s State, err error) error
}

var (
	// Discard is a handler that discards the given error, causing
	// normal control flow to resume.
	Discard = HandlerFunc(discard)

	// Fatal is handler that causes execution to halt..
	Fatal = HandlerFunc(fatal)
)

func discard(s State, err error) error { return nil }

func fatal(s State, err error) error {
	os.Exit(1)
	return nil
}

// The HandlerFunc type is an adapter to allow the use of ordinary functions as
// error handlers. If f is a function with the appropriate signature,
// HandlerFunc(f) is a Handler that calls f.
type HandlerFunc func(s State, err error) error

// Handle calls f(err).
func (f HandlerFunc) Handle(s State, err error) error {
	return f(s, err)
}

type deferHandler func(interface{}) DeferFunc

// DeferSelector registers a handler is used by Auto and AutoDefer to select a
// DeferFunc for a given value. A handler may return nil, in which case the
// next handler or the default types will be attempted.
func DeferSelector(h func(x interface{}) DeferFunc) Option {
	return func(c *Config) {
		c.deferSelectors = append(c.deferSelectors, h)
	}
}

// DefaultHandler adds a hander that is run when an error is detected by an
// error checking call and if this call itself did not specify any handlers.
//
// Multiple default handlers may be specified and they will be called in
// sequence in the order added where the sequence will be terminated if one of
// the handlers returns nil.
func DefaultHandler(h Handler) Option {
	return func(c *Config) {
		c.defaultHandlers = append(c.defaultHandlers, h)
	}
}

// DefaultFunc is short for DefaultHandler(HandlerFunc(f)).
func DefaultFunc(f HandlerFunc) Option {
	return func(c *Config) {
		c.defaultHandlers = append(c.defaultHandlers, HandlerFunc(f))
	}
}
