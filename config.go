// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errd

import "context"

// New returns a new Config with the given options.
func New(options ...Option) *Config {
	c := &Config{}
	c.state.config = c
	for _, o := range options {
		o(c)
	}
	return c
}

// A Config defines a default way to handle errors and options.
type Config struct {
	defaultHandlers []Handler

	// Putting a pre-initialized state in a Config improves performance a tad
	// bit.
	state E

	// inPanic indicates a panic is occurring: a copy of this Config with inPanic
	// set is assigned to the state if a panic occurs. This removes this field
	// from core.
	inPanic bool
}

var config Config

func init() {
	config.state.config = &config
}

// Run starts a new error handling scope. The function returns whenever an error
// is encountered with one of the methods on E.
func (c *Config) Run(f func(e *E)) (err error) {
	state := c.state
	state.deferred = state.buf[:0]
	defer doRecover(&state, &err)
	f(&state)
	// Do defers now to save on an extra defer.
	doDefers(&state, 0)
	if state.err != nil {
		return *state.err
	}
	return nil
}

// RunWithContext starts a new error handling scope. The function returns
// whenever an error is encountered with one of the methods on E.
func (c *Config) RunWithContext(ctxt context.Context, f func(e *E)) (err error) {
	state := c.state
	state.deferred = state.buf[:0]
	state.context = ctxt
	defer doRecover(&state, &err)
	f(&state)
	// Do defers now to save on an extra defer.
	doDefers(&state, 0)
	if state.err != nil {
		return *state.err
	}
	return nil
}

// TODO: consider removing these functions:

// Run calls f with a new E.
//
// Use this method with care, as it does not define a default handler.
func Run(f func(*E)) (err error) {
	return config.Run(f)
}

// RunWithContext  calls f with a new E.
//
// Use this method with care, as it does not define a default handler.
func RunWithContext(ctxt context.Context, f func(*E)) (err error) {
	return config.RunWithContext(ctxt, f)
}
