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

// A closerWithError is an io.Closer that also implements CloseWithError.
type closerWithError interface {
	io.Closer
	CloseWithError(error) error
}

type deferData struct {
	x interface{}
	f DeferFunc
}

// A DeferFunc is used to call cleanup code for x at defer time.
type DeferFunc func(s State, x interface{}) error

// DeferFunc calls f at the end of a Run with x as its argument.
//
// If f returns an error it will be passed to the error handlers.
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

var (
	// Close calls x.Close().
	Close DeferFunc = close

	// CloseWithError calls x.CloseWithError().
	CloseWithError DeferFunc = closeWithError

	// Unlock calls x.Unlock().
	Unlock DeferFunc = unlock
)

func close(s State, x interface{}) error {
	return x.(io.Closer).Close()
}

func closeWithError(s State, x interface{}) error {
	c := x.(closerWithError)
	if err := s.Err(); err != nil {
		return c.CloseWithError(err)
	}
	return c.Close()
}

func unlock(s State, x interface{}) error {
	x.(sync.Locker).Unlock()
	return nil
}

func voidFunc(s State, x interface{}) error {
	x.(func())()
	return nil
}

func voidErrorFunc(s State, x interface{}) error {
	return x.(func() error)()
}

func errorFunc(s State, x interface{}) error {
	x.(func(error))(s.Err())
	return nil
}

func errorErrorFunc(s State, x interface{}) error {
	return x.(func(error) error)(s.Err())
}

// Defer defers a call to x, which may be a function of the form:
//    - func()
//    - func() error
//    - func(error)
//    - func(error) error
// An error returned by any of these functions is passed to the error handlers.
//
// Performance-sensitive applications should use DeferFunc.
func (e *E) Defer(x interface{}, h ...Handler) {
	if x != nil {
		for i := len(h) - 1; i >= 0; i-- {
			e.deferred = append(e.deferred, deferData{h[i], nil})
		}
		var f DeferFunc
		switch x.(type) {
		case func():
			f = voidFunc
		case func() error:
			f = voidErrorFunc
		case func(error):
			f = errorFunc
		case func(error) error:
			f = errorErrorFunc
		default:
			panic(fmt.Errorf(notSupported, x))
		}
		e.deferred = append(e.deferred, deferData{x, f})
	}
}

const notSupported = "errd: type %T not supported by Defer"

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
