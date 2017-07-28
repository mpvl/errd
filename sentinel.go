// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errd

import "io"

// IsSentinel returns true if err is the Sentinel error and false if err is nil.
// It will return from the current Run when another error is encountered.
func (e *E) IsSentinel(sentinel, err error, h ...Handler) bool {
	switch err {
	case sentinel:
		return true
	case nil:
		return false
	}
	return processErrorSentinel(e, err, sentinel, h)
}

// IsEOF is a shortand for IsSentinel(io.EOF, err).
func (e *E) IsEOF(err error, h ...Handler) bool {
	// TODO: copy code from IsSentinel to keep stack depth in which
	// handlers are called the same.
	return e.IsSentinel(io.EOF, err, h...)
}
