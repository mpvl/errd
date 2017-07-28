// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errd

import (
	"errors"
	"testing"
)

type closer struct{ v *string }

func (c *closer) Close() error {
	*c.v = "Close"
	return nil
}

type closerWithError struct{ v *string }

func (c *closerWithError) Close() error {
	*c.v = "CloseNil"
	return nil
}

func (c *closerWithError) CloseWithError(err error) error {
	if err == nil {
		*c.v = "CloseNil"
	} else {
		*c.v = "Close" + err.Error()
	}
	return errors.New("defer error")
}

type unlocker struct{ v *string }

func (u *unlocker) Unlock() { *u.v = "Unlocked" }
func (u *unlocker) Lock()   {}

func TestDefer(t *testing.T) {
	var result string

	h1 := HandlerFunc(func(s State, err error) error {
		result += ":DefErr1"
		return err
	})
	h2 := HandlerFunc(func(s State, err error) error {
		result += ":DefErr2"
		return err
	})
	h3 := HandlerFunc(func(s State, err error) error {
		result += ":DefErr3"
		return err
	})

	testCases := []struct {
		f    func(e *E)
		err  error // body error
		want string
	}{{
		f:    func(e *E) { e.DeferClose(&closer{&result}, h1) },
		want: "Close",
	}, {
		f:    func(e *E) { e.Defer(&closer{&result}, h1) },
		want: "Close",
	}, {
		f:    func(e *E) { e.MustDefer(&closer{&result}, h1) },
		want: "Close",
	}, {
		f: func(e *E) {
			e.Defer(&closerWithError{&result}, h1)
		},
		want: "CloseNil",
	}, {
		f: func(e *E) {
			e.DeferCloseWithError(&closerWithError{&result}, h1)
		},
		want: "CloseNil",
	}, {
		f: func(e *E) {
			e.DeferCloseWithError(&closerWithError{&result})
		},
		err:  errors.New("Error"),
		want: "Close:Error",
	}, {
		f: func(e *E) {
			e.DeferCloseWithError(&closerWithError{&result}, h1)
		},
		err:  errors.New("Error"),
		want: "Close:Error:DefErr1",
	}, {
		f: func(e *E) {
			e.DeferCloseWithError(&closerWithError{&result}, h1, h2, h3)
		},
		err:  errors.New("Error"),
		want: "Close:Error:DefErr1:DefErr2:DefErr3",
	}, {
		f: func(e *E) {
			e.Defer(&closerWithError{&result}, h1, h2, h3)
		},
		err:  errors.New("Error"),
		want: "Close:Error:DefErr1:DefErr2:DefErr3",
	}, {
		f: func(e *E) {
			e.MustDefer(&closerWithError{&result}, h1, h2, h3)
		},
		err:  errors.New("Error"),
		want: "Close:Error:DefErr1:DefErr2:DefErr3",
	}, {
		f: func(e *E) {
			e.DeferUnlock(&unlocker{&result})
		},
		err:  errors.New("Error"),
		want: "Unlocked",
	}, {
		f: func(e *E) {
			e.Defer(&unlocker{&result})
		},
		err:  errors.New("Error"),
		want: "Unlocked",
	}, {
		f: func(e *E) {
			e.MustDefer(&unlocker{&result})
		},
		err:  errors.New("Error"),
		want: "Unlocked",
	}, {
		f: func(e *E) {
			e.DeferFunc(&closerWithError{&result}, closeWithError, h1)
		},
		err:  errors.New("Error"),
		want: "Close:Error:DefErr1",
	}}
	for _, tc := range testCases {
		result = ""
		t.Run(tc.want, func(t *testing.T) {
			Run(func(e *E) {
				tc.f(e)
				e.Must(tc.err, HandlerFunc(func(s State, err error) error {
					return errors.New(":" + err.Error())
				}))
			})
			if result != tc.want {
				t.Errorf("err: got %q; want %q", result, tc.want)
			}
		})
	}
}
