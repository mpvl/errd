// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errd

import (
	"errors"
	"fmt"
)

// Assert causes Run to return if condition is false.
// The msg argument may be an error or a string which will be turned into an
// error. The error will be passed to error handling as with Must.
func (e *E) Assert(condition bool, msg interface{}, h ...Handler) {
	if !condition {
		var err error
		switch x := msg.(type) {
		case error:
			err = x
		case string:
			err = errors.New(x)
		default:
			err = fmt.Errorf("errd: %#v", x)
		}
		processError(e, err, h)
	}
}

// Assertf causes Run to return if condition is false, in which case it will
// create an error from the given formatted message. The error will be passed
// to error handling.
func (e *E) Assertf(condition bool, format string, args ...interface{}) {
	if !condition {
		processError(e, fmt.Errorf(format, args...), nil)
	}
}
