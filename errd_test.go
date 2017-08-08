package errd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

const (
	success = iota
	deferError
	deferPanic
	retError
	bodyPanic
	numAction
)

func actionStr(action int) string {
	switch action {
	case success:
		return "success"
	case deferError:
		return "closeErr"
	case retError:
		return "retErr"
	case bodyPanic:
		return "panic"
	case deferPanic:
		return "panicInDefer"
	}
	panic("ex: unreachable")
}

var (
	testCases = [][]int{
		{success},
		{deferError},
		{deferPanic},
		{retError},
		{bodyPanic},
	}

	retErrors, deferErrors, panicErrors, deferPanicErrors []error
)

func init() {
	// Add new test cases, each time adding one of two digits to each existing
	// one.
	for i := 0; i < 3; i++ {
		prev := testCases
		for j := success; j < retError; j++ {
			for _, tc := range prev {
				testCases = append(testCases, append([]int{j}, tc...))
			}
		}
	}

	// pre-allocate sufficient return and close errors
	for id := 0; id < 10; id++ {
		retErrors = append(retErrors, fmt.Errorf("return-err:%d", id))
		deferErrors = append(deferErrors, fmt.Errorf("close-err:%d", id))
		panicErrors = append(panicErrors, fmt.Errorf("panic-err:%d", id))
		deferPanicErrors = append(deferPanicErrors, fmt.Errorf("defer-panic-err:%d", id))
	}
}

type idCloser struct {
	id     int
	action int
	w      io.Writer
}

func (c *idCloser) Close() error {
	if c.w != nil {
		fmt.Fprintln(c.w, "closed", c.id)
	}
	switch c.action {
	case deferError:
		return deferErrors[c.id]
	case deferPanic:
		panic(deferPanicErrors[c.id])
	}
	return nil
}

func (c *idCloser) CloseWithError(err error) error {
	if c.w != nil {
		if err == nil {
			fmt.Fprintln(c.w, "closed", c.id)
		} else {
			fmt.Fprint(c.w, "closed with error: ", c.id)
			fmt.Fprintln(c.w, ":", err)
		}
	}
	switch c.action {
	case deferError:
		return deferErrors[c.id]
	case deferPanic:
		panic(deferPanicErrors[c.id])
	}
	return nil
}

type errFunc func() error

// TestConformance simulates 1 or more blocks of creating values and handling
// errors and defers and compares the result of using the traditional style Go
// and using package errd.
func TestConformanceDefer(t *testing.T) {
	for _, tc := range testCases {
		t.Run(key(tc), func(t *testing.T) {
			want := simulate(tc, properTraditionalDefer)
			got := simulate(tc, errdClosureDefer)
			if got != want {
				t.Errorf("\n=== got:\n%s=== want:\n%s", got, want)
			}
		})
	}
}

// idiomaticTraditionalDefer is how error handling is usually done manually for
// the case described in TestConformance. However, this approach may miss
// detecting and returning errors and is not correct in the general case.
// We include this set for illustrative purposes, including benchmarks, where
// it can be used to show the cost of doing things more properly.
func idiomaticTraditionalDefer(w io.Writer, actions []int) errFunc {
	closers := make([]idCloser, len(actions))
	return func() error {
		for i, a := range actions {
			c, err := retDefer(w, closers, i, a)
			if err != nil {
				return err
			}
			defer c.Close()
		}
		return nil
	}
}

// properTraditionalDefer goes beyond the common ways in which errors are
// handled and also detects errors resulting from close.
func properTraditionalDefer(w io.Writer, actions []int) errFunc {
	closers := make([]idCloser, len(actions))
	return func() (errOut error) {
		for i, a := range actions {
			c, err := retDefer(w, closers, i, a)
			if err != nil {
				return err
			}
			defer func() {
				if err := c.Close(); err != nil && errOut == nil {
					errOut = err
				}
			}()
		}
		return nil
	}
}

var ec = WithDefault()

func errdClosureDefer(w io.Writer, actions []int) errFunc {
	closers := make([]idCloser, len(actions))
	return func() error {
		err := ec.Run(func(e *E) {
			for i, a := range actions {
				c, err := retDefer(w, closers, i, a)
				e.Must(err)
				e.Defer(c.Close)
			}
		})
		return err
	}
}

func errdFuncDefer(w io.Writer, actions []int) errFunc {
	closers := make([]idCloser, len(actions))
	return func() error {
		err := ec.Run(func(e *E) {
			for i, a := range actions {
				c, err := retDefer(w, closers, i, a)
				e.Must(err)
				e.deferFunc(c, close)
			}
		})
		return err
	}
}

// TestConformanceWithError simulates 1 or more blocks of creating values and
// handling errors and defers, where the deferred function needs to be passed
// *any* earlier occurring error, including those from panics and those
// originating in other defer blocks.
func TestConformanceDeferWithError(t *testing.T) {
	for _, tc := range testCases {
		t.Run(key(tc), func(t *testing.T) {
			want := simulate(tc, pedanticTraditionalDeferWithError)
			got := simulate(tc, errdClosureDeferWithError)
			if got != want {
				t.Errorf("\n=== got:\n%s=== want:\n%s", got, want)
			}
		})
	}
}

// pedanticTraditionalDeferWithError implements a way to catch ALL errors
// preceding a call to CloseWithError, including panics in the body and
// other defers, without using the errd package.
func pedanticTraditionalDeferWithError(w io.Writer, actions []int) errFunc {
	closers := make([]idCloser, len(actions))
	return func() (errOut error) {
		var isPanic bool
		defer func() {
			// We may still have a panic after our last call to defer so catch.
			if r := recover(); r != nil {
				panic(r)
			}
			// Panic again for panics we caught earlier to pass to defers.
			if isPanic {
				panic(errOut)
			}
		}()
		for i, a := range actions {
			c, err := retDeferWithErr(w, closers, i, a)
			if err != nil {
				return err
			}
			defer func() {
				// We need to recover any possible panic to not miss out on
				// passing the panic. Panics override any previous error.
				if r := recover(); r != nil {
					switch v := r.(type) {
					case error:
						errOut = v
					default:
						errOut = fmt.Errorf("%v", v)
					}
					isPanic = true
				}
				if errOut != nil {
					c.CloseWithError(errOut)
				} else {
					if err := c.Close(); err != nil && errOut == nil {
						errOut = err
					}
				}
			}()
		}
		return nil
	}
}

func errdClosureDeferWithError(w io.Writer, actions []int) errFunc {
	closers := make([]idCloser, len(actions))
	return func() error {
		return ec.Run(func(e *E) {
			for i, a := range actions {
				c, err := retDeferWithErr(w, closers, i, a)
				e.Must(err, identity)
				e.Defer(c.CloseWithError)
			}
		})
	}
}

func errdFuncDeferWithError(w io.Writer, actions []int) errFunc {
	closers := make([]idCloser, len(actions))
	return func() error {
		return ec.Run(func(e *E) {
			for i, a := range actions {
				c, err := retDeferWithErr(w, closers, i, a)
				e.Must(err, identity)
				e.deferFunc(c, closeWithErrorFunc)
			}
		})
	}
}

type benchCase struct {
	name string
	f    func(w io.Writer, actions []int) errFunc
}

var testFuncsDeferClose = []benchCase{
	{"idiomatic traditional", idiomaticTraditionalDefer},
	{"proper traditional", properTraditionalDefer},
	{"errd/closer", errdClosureDefer},
}

var testFuncsDeferCloseWithError = []benchCase{
	{"traditional/closerwe", pedanticTraditionalDeferWithError},
	{"errd/closerwe", errdClosureDeferWithError},
}

var testFuncsNoDefer = []benchCase{
	{"traditional", traditionalCheck},
	{"errd", errdClosureCheck},
}

func traditionalCheck(w io.Writer, actions []int) errFunc {
	return func() error {
		for i, a := range actions {
			err := retNoDefer(w, i, a)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

func errdClosureCheck(w io.Writer, actions []int) errFunc {
	return func() error {
		return ec.Run(func(e *E) {
			for i, a := range actions {
				e.Must(retNoDefer(w, i, a))
			}
		})
	}
}

func retDefer(w io.Writer, closers []idCloser, id, action int) (io.Closer, error) {
	// pre-allocate io.Closers. This is not realistice, but eliminates this
	// allocation from the measurements.
	closers[id] = idCloser{id, action, w}
	switch action {
	case success, deferError, deferPanic:
		return &closers[id], nil
	case retError:
		return nil, retErrors[id]
	case bodyPanic:
		panic(panicErrors[id])
	}
	panic("errd: unreachable")
}

func retDeferWithErr(w io.Writer, closers []idCloser, id, action int) (closerWithError, error) {
	// pre-allocate io.Closers. This is not realistice, but eliminates this
	// allocation from the measurements.
	closers[id] = idCloser{id, action, w}
	switch action {
	case success, deferError, deferPanic:
		return &closers[id], nil
	case retError:
		return nil, retErrors[id]
	case bodyPanic:
		panic(panicErrors[id])
	}
	panic("errd: unreachable")
}

func retNoDefer(w io.Writer, id, action int) error {
	// pre-allocate io.Closers. This is not realistice, but eliminates this
	// allocation from the measurements.
	switch action {
	case success:
		return nil
	case retError:
		return retErrors[id]
	case bodyPanic:
		panic(panicErrors[id])
	}
	panic("errd: unreachable")
}

func key(test []int) (key string) {
	s := []string{}
	for _, a := range test {
		s = append(s, actionStr(a))
	}
	return strings.Join(s, "-")
}

func simulate(actions []int, f func(w io.Writer, actions []int) errFunc) (result string) {
	w := &bytes.Buffer{}
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintf(w, "P:%v\n", err)
		}
		result = w.String()
	}()
	fmt.Fprintln(w, f(w, actions)())
	return "" // set in defer
}

var benchCases = []struct {
	allocs  int // number of extra allocs
	actions []int
}{
	{0, []int{success}},
	{0, []int{success, success}},
	{0, []int{success, success, success}},
	{1, []int{success, success, success, success}},
	// Uncomment these for more benchmarks.
	// {1, []int{success, success, success, success, success}},
	// {0, []int{retError}},
	// {0, []int{success, retError}},
	// {0, []int{success, success, retError}},
	// {0, []int{success, success, success, retError}},
	// {1, []int{success, success, success, success, retError}},
}

func runBenchCases(b *testing.B, bf []benchCase) {
	for _, bc := range benchCases {
		for _, bf := range bf {
			b.Run(key(bc.actions)+"/"+bf.name, func(b *testing.B) {
				f := bf.f(nil, bc.actions)
				for i := 0; i < b.N; i++ {
					f()
				}
			})
		}
	}
}

func BenchmarkNoDefer(b *testing.B) {
	runBenchCases(b, testFuncsNoDefer)
}

func BenchmarkDeferClose(b *testing.B) {
	runBenchCases(b, testFuncsDeferClose)
}

func BenchmarkDeferCloseWithError(b *testing.B) {
	runBenchCases(b, testFuncsDeferCloseWithError)
}

var testAllocsDefer = []benchCase{
	{"errd/closer", errdFuncDefer},
	{"errd/closerwe", errdFuncDeferWithError},
}

func TestAlloc(t *testing.T) {
	allFuncs := append(testFuncsNoDefer, testAllocsDefer...)
	for _, tf := range allFuncs {
		for _, bc := range benchCases {
			t.Run(key(bc.actions)+"/"+tf.name, func(t *testing.T) {
				if !strings.HasPrefix(tf.name, "errd") {
					return
				}
				f := tf.f(nil, bc.actions)
				got := testing.AllocsPerRun(10, func() {
					f()
				})
				want := 1
				if strings.Contains(tf.name, "closer") {
					want += bc.allocs
				}
				if int(got) != want {
					t.Errorf("got %v; want %v", got, want)
				}
			})
		}
	}
}

func TestRunWithContext(t *testing.T) {
	var ctx context.Context
	h := HandlerFunc(func(s State, err error) error {
		ctx = s.Context()
		return err
	})

	Run(func(e *E) { e.Must(errors.New("no context"), h) })
	if ctx == nil {
		t.Error("got a nil context, expected TODO")
	}

	errContext := errors.New("context")
	bg := context.Background()
	RunWithContext(bg, func(e *E) { e.Must(errContext, h) })
	if ctx != bg {
		t.Errorf("got %v; expect defined background context", ctx)
	}

	err := RunWithContext(bg, func(e *E) {
		e.Defer(func() error { return errContext }, h)
	})
	if err != errContext {
		t.Errorf("got %v; want %v", err, errContext)
	}
}

func TestPanic(t *testing.T) {
	errFoo := errors.New("foo")
	testCases := []struct {
		f       func(e *E)
		p       interface{}
		err     string
		noPanic bool
	}{{
		f: func(e *E) {
		},
		p:       nil,
		noPanic: true,
	}, {
		f: func(e *E) {
			panic("bar")
		},
		p:   "bar",
		err: "errd: paniced: bar",
	}, {
		f: func(e *E) {
			panic(errFoo)
		},
		p:   errFoo,
		err: "foo",
	}, {
		f: func(e *E) {
			panic(2)
		},
		p:   2,
		err: "errd: paniced: 2",
	}, {
		f: func(e *E) {
			e.deferFunc(nil, nil) // panic: nil func
		},
		p:   errNilFunc,
		err: errNilFunc.Error(),
	}, {
		f: func(e *E) {
			e.Defer(1) // panic: not supported
		},
		err: "errd: type int not supported by Defer",
	}}
	for _, tc := range testCases {
		paniced := false
		ec := WithDefault(HandlerFunc(func(s State, err error) error {
			paniced = s.Panicking()
			return err
		}))
		t.Run("", func(t *testing.T) {
			defer func() {
				r := recover()
				if tc.p != nil && (r == nil || r != nil && r != tc.p) {
					t.Errorf("got %v; want %v", r, tc.p)
				}
				if paniced != !tc.noPanic {
					t.Errorf("got %v; want %v", paniced, !tc.noPanic)
				}
			}()
			ec.Run(func(e *E) {
				e.deferFunc(nil, func(s State, x interface{}) error {
					err := s.Err()
					if err == nil && tc.err != "" || err != nil && err.Error() != tc.err {
						t.Errorf("got %q; want %q", err, tc.err)
					}
					return err
				})
				tc.f(e)
			})
		})
	}
}
