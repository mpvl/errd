// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errd

import (
	"os"
)

// A Handler processes errors.
type Handler interface {
	Handle(s State, err error) error
}

var (
	// Discard is a handler that discards the given error, causing
	// normal control flow to resume.
	Discard Handler = HandlerFunc(discard)

	// Fatal is handler that causes execution to halt.
	Fatal Handler = HandlerFunc(fatal)
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

// Handle calls f(s, err).
func (f HandlerFunc) Handle(s State, err error) error {
	return f(s, err)
}
