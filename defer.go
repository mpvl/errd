// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errd

import (
	"errors"
	"fmt"
	"io"
	"sync"
)

// A CloserWithError is an io.Closer that also implements CloseWithError.
type CloserWithError interface {
	io.Closer
	CloseWithError(error) error
}

type deferData struct {
	x interface{}
	f DeferFunc
}

// A DeferFunc is used to call cleanup code for x at defer time.
type DeferFunc func(s State, x interface{}) error

// DeferFunc calls f(x, err) at the end of a Run, where err is the current
// error.
//
// If f returns an error even if an error already existed, the previously
// existing error is retained to be returned by Run. In either case, the error
// will be passed to the error handler.
func (e *E) DeferFunc(x interface{}, f DeferFunc, h ...Handler) {
	if f == nil {
		panic(errNilFunc)
	}
	for i := len(h) - 1; i >= 0; i-- {
		e.deferred = append(e.deferred, deferData{h[i], nil})
	}
	e.deferred = append(e.deferred, deferData{x, f})
}

var errNilFunc = errors.New("errd: nil DeferFunc")

// DeferClose defers a call to x.Close.
func (e *E) DeferClose(x io.Closer, h ...Handler) {
	for i := len(h) - 1; i >= 0; i-- {
		e.deferred = append(e.deferred, deferData{h[i], nil})
	}
	e.deferred = append(e.deferred, deferData{x, close})
}

// DeferCloseWithError defers a call to x.CloseWithError if an error was
// encountered or x.Close otherwise.
func (e *E) DeferCloseWithError(x CloserWithError, h ...Handler) {
	for i := len(h) - 1; i >= 0; i-- {
		e.deferred = append(e.deferred, deferData{h[i], nil})
	}
	e.deferred = append(e.deferred, deferData{x, closeWithError})
}

// DeferUnlock defers a call to x.Unlock.
func (e *E) DeferUnlock(x sync.Locker) {
	e.deferred = append(e.deferred, deferData{x, unlock})
}

// TODO: expose this?
// var (
// 	// Close calls x.Close().
// 	Close DeferFunc = close
//
// 	// CloseWithError
// 	CloseWithError DeferFunc = closeWithError
//
// 	// Unlock calls x.Unlock().
// 	Unlock DeferFunc = unlock
// )

func close(s State, x interface{}) error {
	return x.(io.Closer).Close()
}

func closeWithError(s State, x interface{}) error {
	c := x.(CloserWithError)
	if err := s.Err(); err != nil {
		return c.CloseWithError(err)
	}
	return c.Close()
}

func unlock(s State, x interface{}) error {
	x.(sync.Locker).Unlock()
	return nil
}

// Defer calls a defer on x based on its type. Defer panics the type of x is
// not supported.
//
// Defer is configured to pick the most conservative approach to cleaning up
// by default. It picks the following defer methods for these types:
//    - CloserWithError:      DeferCloseWithError
//    - io.Closer:            DeferClose
//    - sync.Locker           DeferUnlock
//
// TODO: support func(), func() error, func(error), and func(error) error to
// allow closures of existing methods, with a caveat that this tends to be slow.
//
// Additional types can be supported using the DeferSelector Option.
func (e *E) Defer(x interface{}, h ...Handler) {
	if x != nil {
		for i := len(h) - 1; i >= 0; i-- {
			e.deferred = append(e.deferred, deferData{h[i], nil})
		}
		var f DeferFunc
	outer:
		switch x.(type) {
		case CloserWithError:
			f = closeWithError
		case io.Closer:
			f = close
		case sync.Locker:
			f = unlock
		default:
			for _, sel := range e.config.deferSelectors {
				if f = sel(x); f != nil {
					break outer
				}
			}
			panic(fmt.Errorf(notSupported, x))
		}
		e.deferred = append(e.deferred, deferData{x, f})
	}
}

const notSupported = "errd: type %T not supported by Defer"

func (e *E) autoDefer(x interface{}, handlers []interface{}) {
	if x != nil {
		first := len(e.deferred)
		for _, x := range handlers {
			h, ok := x.(Handler)
			if !ok {
				break
			}
			e.deferred = append(e.deferred, deferData{h, nil})
		}
		for j, k := first, len(e.deferred)-1; j < k; {
			e.deferred[j], e.deferred[k] = e.deferred[k], e.deferred[j]
			j++
			k--
		}
		var f DeferFunc
	outer:
		switch x.(type) {
		case CloserWithError:
			f = closeWithError
		case io.Closer:
			f = close
		case sync.Locker:
			f = unlock
		default:
			for _, h := range e.config.deferSelectors {
				if f = h(x); f != nil {
					break outer
				}
			}
			panic(fmt.Errorf(notSupported, x))
		}
		e.deferred = append(e.deferred, deferData{x, f})
	}
}

// TODO
//
// // DeferScope calls f and calls all defers that were added within that call
// // after it completes. An error that occurs in f is handled as if the error
// // occurred in the caller. This includes errors in defer. DeferScope is used to
// // force early cleanup of defers within a tight loop.
// func (e *E) DeferScope(f func()) {
// 	localDefer := len(e.deferred)
// 	f()
// 	doDefers(e, localDefer)

// 	// TODO: bail if we detect an error.
// }
