// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errd_test

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mpvl/errd"
)

func ExampleHandler_fatal() {
	exitOnError := errd.New(errd.DefaultHandler(errd.Fatal))
	exitOnError.Run(func(e *errd.E) {
		r, err := newReader()
		e.Must(err)
		e.Defer(r.Close)

		r, err = newFaultyReader()
		e.Must(err)
		e.Defer(r.Close)
	})
}

func newReader() (io.ReadCloser, error) {
	return ioutil.NopCloser(strings.NewReader("Hello World!")), nil
}

func newFaultyReader() (io.ReadCloser, error) {
	return nil, errors.New("errd_test: error")
}

func ExampleRun() {
	errd.Run(func(e *errd.E) {
		r, err := newReader() // contents: Hello World!
		e.Must(err)
		e.Defer(r.Close)

		_, err = io.Copy(os.Stdout, r)
		e.Must(err)
	})
	// Output:
	// Hello World!
}

func ExampleRun_pipe() {
	r, w := io.Pipe()
	go errd.Run(func(e *errd.E) {
		e.Defer(w)

		r, err := newReader() // contents: Hello World!
		e.Must(err)
		e.Defer(r.Close)

		_, err = io.Copy(w, r)
		e.Must(err)
	})
	io.Copy(os.Stdout, r)

	// The above goroutine is equivalent to:
	//
	// go func() {
	// 	var err error                // used to intercept downstream errors
	// 	defer w.CloseWithError(err)
	//
	// 	r, err := newReader()
	// 	if err != nil {
	// 		return
	// 	}
	// 	defer func() {
	// 		err = r.Close()
	// 	}
	//
	// 	_, err = io.Copy(w, r)
	// }()

	// Output:
	// Hello World!
}

func do(ctx context.Context) {
	// do something
}

// ExampleE_Defer_cancelHelper shows how a helper function may call a
// defer in the caller's E.
func ExampleE_Defer_cancelHelper() {
	contextWithTimeout := func(e *errd.E, req *http.Request) context.Context {
		var cancel context.CancelFunc
		ctx := req.Context()
		timeout, err := time.ParseDuration(req.FormValue("timeout"))
		if err == nil {
			// The request has a timeout, so create a context that is
			// canceled automatically when the timeout expires.
			ctx, cancel = context.WithTimeout(ctx, timeout)
		} else {
			ctx, cancel = context.WithCancel(ctx)
		}
		e.Defer(cancel)
		return ctx
	}

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		errd.Run(func(e *errd.E) {
			ctx := contextWithTimeout(e, req)

			do(ctx)
		})
	})
}
