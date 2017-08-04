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

type closerError struct{ v *string }

func (c *closerError) Close() error {
	*c.v = "CloseNil"
	return nil
}

func (c *closerError) CloseWithError(err error) error {
	if err == nil {
		*c.v = "CloseNil"
	} else {
		*c.v = "Close:" + err.Error()
	}
	return errors.New("defer error")
}

type locker struct{ v *string }

func (u *locker) Unlock() { *u.v = "Unlocked" }
func (u *locker) Lock()   {}

type errInOnly struct{ v *string }

func (e *errInOnly) Abort(err error) { *e.v = "Abort" }

type wrapError struct {
	error
}

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

	errTest := errors.New("Error")
	errWrap := wrapError{errTest}
	closer := &closer{&result}
	closerError := &closerError{&result}
	locker := &locker{&result}
	errInOnly := &errInOnly{&result}
	testCases := []struct {
		f           func(e *E)
		err         error // body error
		wrapped     error
		want        string
		defHandlers []Handler
	}{{
		f:    func(e *E) { e.Defer(closer.Close, h1) },
		want: "Close",
	}, {
		f: func(e *E) {
			e.deferFunc(closerError, closeWithErrorFunc, h1)
		},
		want: "CloseNil",
	}, {
		f: func(e *E) {
			e.Defer(closerError.CloseWithError, h1, h2, h3)
		},
		err:     errTest,
		wrapped: errWrap,
		want:    "Close:Error:DefErr1:DefErr2:DefErr3",
	}, {
		f: func(e *E) {
			e.Defer(locker.Unlock)
		},
		err:     errTest,
		wrapped: errWrap,
		want:    "Unlocked",
	}, {
		f: func(e *E) {
			e.Defer(errInOnly.Abort)
		},
		err:     errTest,
		wrapped: errWrap,
		want:    "Abort",
	}, {
		f: func(e *E) {
			e.Defer(func(s State) error {
				result += "State"
				return s.Err()
			})
		},
		err:     errTest,
		wrapped: errWrap,
		want:    "State",
	}, {
		f: func(e *E) {
			e.Defer(func(s State) error {
				return errors.New("to discard")
			}, Discard)
		},
	}, {
		f: func(e *E) {
			e.Defer(func(s State) error {
				return errors.New("to discard")
			})
		},
		defHandlers: []Handler{Discard},
	}, {
		f: func(e *E) {
			e.deferFunc(closerError, closeWithError, h1)
		},
		err:     errTest,
		wrapped: errWrap,
		want:    "Close:Error:DefErr1",
	}, {
		f: func(e *E) {
			e.deferFunc(locker, unlock, h1)
		},
		err:     errTest,
		wrapped: errWrap,
		want:    "Unlocked",
	}}
	for _, tc := range testCases {
		result = ""
		t.Run(tc.want, func(t *testing.T) {
			err := WithDefault(tc.defHandlers...).Run(func(e *E) {
				tc.f(e)
				e.Must(tc.err, HandlerFunc(func(s State, err error) error {
					return wrapError{err}
				}))
			})
			if err != tc.wrapped {
				t.Errorf("err: got %q; want %q", err, tc.wrapped)
			}
			if result != tc.want {
				t.Errorf("result: got %q; want %q", result, tc.want)
			}
		})
	}
}

func BenchmarkDeferFunc(b *testing.B) {
	x := &closer{}
	ec.Run(func(e *E) {
		for i := 0; i < b.N; i++ {
			e.deferFunc(x, closeFunc)
			e.deferred = e.deferred[:0]
		}
	})
}

func BenchmarkDeferClosure(b *testing.B) {
	x := &closer{}
	ec.Run(func(e *E) {
		for i := 0; i < b.N; i++ {
			e.Defer(x.Close)
			e.deferred = e.deferred[:0]
		}
	})
}
