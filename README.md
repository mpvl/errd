# errd [![GoDoc](https://godoc.org/github.com/mpvl/errd?status.svg)](http://godoc.org/github.com/mpvl/errd) [![Report card](https://goreportcard.com/badge/github.com/mpvl/errd)](https://goreportcard.com/report/github.com/mpvl/errd) [![Travis-CI](https://travis-ci.org/mpvl/errd.svg)](https://travis-ci.org/mpvl/errd)


Package `errd` simplifies error and defer handling.

```go get github.com/mpvl/errd```


## Overview

A common pattern in Go after creating new value is to check for any error and
then to call the cleanup method or function for this value immediately following
this check using defer:

```go
w, err := NewWriter()
if err != nil {
    return err
}
defer w.Close()
```

To some Go programmers this is too much typing and several proposals to add
language support for handling errors have been made.
At the same time, users are encouraged to add information to each point
at which an error is encountered.
Many language proposals make it harder to decorate errors, not easier.

Another issue with the idiomatic approach to error handling is that it is
not as straightforward as is it may seem.
For example, it is not uncommon to ignore the error returned by close,
even though it may return useful information.
Things get even more hairy if it becomes important to pass an error to
a defer or if panics need to be caught.
Package `errd` aims to streamlining and simplifying defer handling, making it
easier to wrap errors, all while reducing the amount of typing.

The following piece of idiomatic Go writes the contents of a reader to a file
on Google Cloud Storage:

```go
func writeToGS(ctx context.Context, bucket, dst string, r io.Reader) (err error) {
    client, err := storage.NewClient(ctx)
    if err != nil {
        return err
    }
    defer client.Close()

    w := client.Bucket(bucket).Object(dst).NewWriter(ctx)
    defer func() {
        if r := recover(); r != nil {
            w.CloseWithError(fmt.Errorf("panic: %v", r))
            panic(r)
        }
        if err != nil {
            _ = w.CloseWithError(err)
        } else {
            err = w.Close()
        }
    }
    _, err = io.Copy(w, r)
    return err
}
```

Google Cloud Storage allows files to be written atomically. This code
minimizes the chance of writing a bad file by aborting the write, using
CloseWithError, whenever any anomaly is encountered.
This includes a panic that could occur in the reader.
Note that, in this case, we chose to ignore the error of the first close:
once the second Close is called successfully, the write operation
completed successfully and we consider further errors irrelevant.

This example shows that error handling can be subtle and not the mindless
check-decorate-return pattern it often seems to be.
Does a Closer also support CloseWithError? Which panics need to be handled?
Also note the subtle use of the return variable to convey an error in Copy to
the last defer function.

In package `errd` the same code is written as:

```go
func writeToGS(ctx context.Context, bucket, dst, src string) error {
    return errd.Run(func(e *errd.E) {
        client, err := storage.NewClient(ctx)
        e.Must(err)
        e.Defer(client.Close, errd.Discard)

        w := client.Bucket(bucket).Object(dst).NewWriter(ctx)
        e.Defer(w.CloseWithError)

        _, err = io.Copy(w, r)
        e.Must(err)
    })
}
```

`Run` starts a scope in which errors and defers are managed.
The `Must` forces the closure called by Run to return immediately if the passed
error is not nil.
The `Defer` methods implement a conservative handling of errors, passing
along any caught error when possible.


## Error Handlers

In all of the above code we made the common faux pas of passing errors on
without decorating them. Package `errd` defines a Handler type to simplify the
task of decorating.

Suppose we want to use `github.com/pkg/errors` to decorate errors.
A simple handler can be defined as:

```go
type msg string

func (m msg) Handle(s errd.State, err error) error {
    return errors.WithMessage(err, string(m))
}
```

This handler can then be used as follows:

```go
func writeToGS(ctx context.Context, bucket, dst, src string) error {
    return errd.Run(func(e *errd.E) {
        client, err := storage.NewClient(ctx)
        e.Must(err, msg("error opening client"))
        e.Defer(client.Close)

        w := client.Bucket(bucket).Object(dst).NewWriter(ctx)
        e.Defer(w.CloseWithError)

        _, err = io.Copy(w, r)
        e.Must(err, msg("error copying contents"))
    })
}
```

Instead of handling individual cases, we can also define a more generic storage
error handle to add information based on the returned error.
This code uses such a default handler:

```go
var ecGS = errd.New(convertGSError)

func writeToGS(ctx context.Context, bucket, dst, src string) error {
    return ecGS.Run(func(e *errd.E) {
        client, err := storage.NewClient(ctx)
        e.Must(err)
        e.Defer(client.Close)
        ...
    })
}
```

The use of default handlers makes it easier to ensure an error is always
handled.


## How it works

Package `errd` uses Go's `panic` and `recover` mechanism to force the exit from
`Run` if an error is encountered.
On top of that, package `errd` manages its own defer state, which is
necessary to properly interweave error and defer handling.


## Naming

The name of the package suggests this is a daemon for handling errors.
The package is clearly not a daemon.
However, `errd` does suggests that it is does something with errors related to
handling them in the background.
But that is pretty much what the package does and it is short to boot.
The `errd` name originally popped in my head as a result of finding contractions
of error and defer.
So if the daemon implication bothers you, think of the `d` standing for
`defer`.

`Must` is idiomatically used in Go to indicate code will panic, and thus
abort the usual flow of execution, if an error is encountered.
We piggyback on this convention to signal that the execution of the closure
passed to `Run` will be interrupted if a non-nil error is passed to `Must`.
Where package `errd` differs from the convention is that it catches the panic
created by `Must` and converts it to an error when `Run` returns.
A similar effect can be achieved for a `Must` of any other package by catching
the panic with a `defer` an `recover`.
As such package `errd` merely expands this convention.


## Performance

Package `errd` adds a defer block to do all its management.
If the original code only does error checking, this is a relatively
big price to pay.
If the original code already does a defer, the damage is limited.
If the original code uses multiple defers, package `errd` may even be faster.

Passing string-type error handlers, like in the example on error handlers,
causes an allocation.
However, in 1.9 this special case does not incur noticeable overhead over
passing a pre-allocated handler.


## Design Philosophy

The package aims to simplify error and defer handling now, with an
ulterior motive to gather enough data to aid making decisions on how to improve
error and defer handling in Go 2.

Some design guidelines used in the development of this package:

- Stay true to the "error are values" principle.
- Make the most conservative error handling the default,
  and allow users to be explicit about relaxing the rules.
- Panics stay panics: although we use the panic mechanism, do not let the user
  use this mechanism to bypass panics.
- Stay compatible with existing packages for extending and adorning errors
  (like github.com/pkg/errors).
- Performance: don't do things that impact performance too much: allow as
  many people as possible to use the package now, versus relying on possible
  compiler improvements down the road.


## What's next
Package `errd` is about exploring better ways and semantics for handling errors
and defers with an aim to improve Go 2.

_NOTE: the rest of this section is just brainstorming and is no indication
of where Go is heading._

For example, The approach of package `errd` interacts well with generics.

Consider the methods:
```
func [T] (e *E) Defer(x T, h ... Handler) T
func [T] (e *E) MustD1(x T, err error, h ... Handler) T
```
Where `MustD1` checks the error and returns and the first argument after first
adding the defer handler for this argument.
The `writeToGS` function from above can now be written as:

```go
func writeToGS(ctx context.Context, bkt, dst, src string) error {
    return errd.Run(func(e *errd.E) {
        client := e.MustD1(storage.NewClient(ctx), msg("error opening client"))
        _ = e.Must1(io.Copy(e.Defer(client.Bucket(bkt).Object(dst).NewWriter(ctx)), r))
    })
}
```

Syntactic sugar could further simplify things:

```go
func writeToGS(ctx context.Context, bkt, dst, src string) error {
    return catch(handler) {
        client := storage.close#must#NewClient(ctx)
        _ = io.must#Copy(client.Bucket(bkt).Object(dst).close#NewWriter(ctx), r)
    })
}
```

where `must` checks the error and bails if non-nil, `close` calls a defer func
that picks `Close` and `CloseWithError`.
Both `must` and `close` could be user-defined wrapper functions.

A big advantage of making this a language feature is that it would be easier
to enforce that this feature is not used across function, or even worse,
goroutine boundaries.


## Related Issues and Posts
https://blog.golang.org/errors-are-values

https://research.swtch.com/go2017#error

In https://github.com/golang/go/issues/20803 argues against allowing implicitly
ignored variables.
Package `errd`  approach tackles a similar sentiment by making it more likely
an error is handled by default and requiring more intervention to ignore
an error.

Issue https://github.com/golang/go/issues/18721 aims to be terser by introducing
syntactic sugar for error handling. Error handling as done by package `errd`,
though terser, is clearly not as terse.
However, the hope is that building experience with the use of package `errd`
helps making the right decision on what kind of language changes are appropriate.
One of the cons mentioned in this issue is that it may encourage bad practices
such as not decorating errors with additional information.
Package `errd` makes such decoration easier, not harder.

Issue https://github.com/golang/go/issues/16225 also expresses the desire to do
shorter and terser error handling.
Package `errd` accomplishes this without a language change.

Some other related Issues:
https://github.com/golang/go/issues/19511
https://github.com/golang/go/issues/19727
https://github.com/golang/go/issues/20148
