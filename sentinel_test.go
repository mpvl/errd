// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errd

import (
	"errors"
	"io"
	"testing"
)

var errOther = errors.New("other")

var toEOF = HandlerFunc(func(s State, err error) error { return io.EOF })

func TestIsSentinel(t *testing.T) {
	testCases := []struct {
		err      error
		want     bool
		wantErr  error
		handlers []Handler
	}{{
		err:     nil,
		want:    false,
		wantErr: nil,
	}, {
		err:     io.EOF,
		want:    true,
		wantErr: nil,
	}, {
		err:      io.EOF,
		want:     true,
		wantErr:  nil,
		handlers: []Handler{Discard},
	}, {
		err:     errOther,
		want:    true, // not checked!
		wantErr: errOther,
	}, {
		err:     errOther,
		want:    false, // not checked!
		wantErr: errOther,
	}, {
		err:      errOther,
		want:     false,
		wantErr:  nil,
		handlers: []Handler{Discard},
	}, {
		err:      errOther,
		want:     true,
		wantErr:  nil,
		handlers: []Handler{toEOF},
	}}
	for i, tc := range testCases {
		err := Run(func(e *E) {
			got := e.IsEOF(tc.err, tc.handlers...)
			if got != tc.want {
				t.Errorf("%d:IsEOF:bool: got %v; want %v", i, got, tc.want)
			}
		})
		if err != tc.wantErr {
			t.Errorf("%d:IsEOF:err: got %v; want %v", i, err, tc.wantErr)
		}
		err = Run(func(e *E) {
			got := e.IsSentinel(io.EOF, tc.err, tc.handlers...)
			if got != tc.want {
				t.Errorf("%d:IsSentinel:bool: got %v; want %v", i, got, tc.want)
			}
		})
		if err != tc.wantErr {
			t.Errorf("%d:IsSentinel:err: got %v; want %v", i, err, tc.wantErr)
		}
		err = New(tc.handlers...).Run(func(e *E) {
			got := e.IsEOF(tc.err)
			if got != tc.want {
				t.Errorf("%d:IsEOF:bool: got %v; want %v", i, got, tc.want)
			}
		})
		if err != tc.wantErr {
			t.Errorf("%d:IsEOF:err: got %v; want %v", i, err, tc.wantErr)
		}

	}
}
