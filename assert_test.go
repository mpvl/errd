// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errd

import (
	"errors"
	"testing"
)

func TestAssert(t *testing.T) {
	testCases := []struct {
		test     bool
		msg      interface{}
		handlers []Handler
		err      string
	}{{
		test: true,
		msg:  "error",
		err:  "",
	}, {
		test: false,
		msg:  "error",
		err:  "error",
	}, {
		test: false,
		msg:  errors.New("error"),
		err:  "error",
	}, {
		test: false,
		msg:  3,
		err:  "errd: 3",
	}, {
		test:     false,
		msg:      "error",
		handlers: []Handler{Discard},
	}}
	for i, tc := range testCases {
		err := Run(func(e *E) { e.Assert(tc.test, tc.msg, tc.handlers...) })
		if err == nil && tc.err != "" || err != nil && err.Error() != tc.err {
			t.Errorf("%d: got %q; want %q", i, err, tc.err)
		}
	}
}

func TestAssertf(t *testing.T) {
	testCases := []struct {
		test   bool
		format string
		args   []interface{}
		err    string
	}{{
		test:   false,
		format: "err",
		err:    "err",
	}, {
		test:   true,
		format: "err",
	}, {
		test:   false,
		format: "format %d",
		args:   []interface{}{1},
		err:    "format 1",
	}, {
		test:   true,
		format: "format %d",
		args:   []interface{}{1},
	}}
	for i, tc := range testCases {
		err := Run(func(e *E) { e.Assertf(tc.test, tc.format, tc.args...) })
		if err == nil && tc.err != "" || err != nil && err.Error() != tc.err {
			t.Errorf("%d: got %q; want %q", i, err, tc.err)
		}
	}
}
