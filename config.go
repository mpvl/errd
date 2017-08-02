// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errd

import "context"

// WithDefault returns a new Config for the given default handlers.
func WithDefault(h ...Handler) *Runner {
	return &Runner{
		config: &config{
			defaultHandlers: h,
		},
	}
}

type config struct {
	defaultHandlers []Handler

	// inPanic indicates a panic is occurring: a copy of this Config with inPanic
	// set is assigned to the state if a panic occurs. This removes this field
	// from core.
	inPanic bool
}

// A Runner defines a default way to handle errors and options.
type Runner struct {
	*config
	context context.Context
}

// Default is the default Runner comfiguration.
var Default = WithDefault()

// Run starts a new error handling scope. The function returns whenever an error
// is encountered with one of the methods on E.
func (r *Runner) Run(f func(e *E)) (err error) {
	// state := c.state
	var e E
	e.runner = r.config
	e.deferred = e.buf[:0]
	defer doRecover(&e, &err)
	f(&e)
	doDefers(&e, 0)
	if e.err != nil {
		return *e.err
	}
	return nil
}

// RunWithContext starts a new error handling scope. The function returns
// whenever an error is encountered with one of the methods on E.
func (r *Runner) RunWithContext(ctxt context.Context, f func(e *E)) (err error) {
	// Do defers now to save on an extra defer.
	// state := c.state
	var e E
	e.runner = r.config
	e.deferred = e.buf[:0]
	e.context = ctxt
	defer doRecover(&e, &err)
	f(&e)
	// Do defers now to save on an extra defer.
	doDefers(&e, 0)
	if e.err != nil {
		return *e.err
	}
	return nil
}

// Run calls Default.Run(f)
func Run(f func(*E)) (err error) {
	return Default.Run(f)
}

// RunWithContext calls Default.RunWithContext(ctxt, f)
func RunWithContext(ctxt context.Context, f func(*E)) (err error) {
	return Default.RunWithContext(ctxt, f)
}
