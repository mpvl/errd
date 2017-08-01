// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package errd simplifies error and defer handling.
//
// Overview
//
// Package errd allows returning form a block of code without using the usual if
// clauses, while properly intercepting errors and passing them to code called
// at defer time.
//
// The following piece of idiomatic Go writes the contents of a reader to a file
// on Google Cloud Storage:
//
//     func writeToGS(ctx context.Context, bucket, dst string, r io.Reader) (err error) {
//         client, err := storage.NewClient(ctx)
//         if err != nil {
//             return err
//         }
//         defer client.Close()
//
//         w := client.Bucket(bucket).Object(dst).NewWriter(ctx)
//         defer func() {
//             if r := recover(); r != nil {
//                 w.CloseWithError(fmt.Errorf("panic: %v", r))
//                 panic(r)
//             }
//             if err != nil {
//                 _ = w.CloseWithError(err)
//             } else {
//                 err = w.Close()
//             }
//         }
//         _, err = io.Copy(w, r)
//         return err
//     }
//
// Google Cloud Storage allows files to be written atomically. This code
// minimizes the chance of writing a bad file by aborting the write, using
// CloseWithError, whenever any anomaly is encountered. This includes a
// panic that could occur in the reader.
//
// Package errd aims to reduce bugs resulting from such subtleties by
// making the default of having very strict error checking easy.
// The following code achieves the same as the above:
//
//     func writeToGS(ctx context.Context, bucket, dst, src string) error {
//         return errd.Run(func(e *errd.E) {
//             client, err := storage.NewClient(ctx)
//             e.Must(err)
//             e.Defer(client.Close, errd.Discard)
//
//             w := client.Bucket(bucket).Object(dst).NewWriter(ctx)
//             e.Defer(w.CloseWithError)
//
//             _, err = io.Copy(w, r)
//             e.Must(err)
//         })
//     }
//
// Discard is an example of an error handler. Here it signals that we want
// to ignore the error of the first Close.
// More on error handlers later.
//
//
// Deferring
//
// Package errd also allows automatic selection of the defer method:
//
//     func writeToGS(ctx context.Context, bucket, dst, src string) error {
//         return errd.Run(func(e *errd.E) {
//             client, err := storage.NewClient(ctx)
//             e.Must(err)
//             e.Defer(client, errd.Discard)
//
//             w := client.Bucket(bucket).Object(dst).NewWriter(ctx)
//             e.Defer(w)
//
//             _, err = io.Copy(w, r)
//             e.Must(err)
//         })
//     }
//
// Defer picks CloseWithError over Close when possible, in line with defaulting
// to the most conservative strategy.
// Users may provide support for additional types with the DeferSelector Option.
//
// Performance-sensitive applications should consider the use of
// the DeferFunc method.
// Package errd includes predefined defer functions for unlocking sync.Lockers
// and closing io.Closers and Closers that include a CloseWithError method.
//
//
// Error Handlers
//
// In all of the code above we made the common faux pas of passing errors on
// without decorating them. Package errd defines a Handler type to simplify the
// task of decorating.
//
// Suppose we want to use github.com/pkg/errors to decorate errors. A simple
// handler can be defined as:
//
//     type msg string
//
//     func (m msg) Handle(s errd.State, err error) error {
//         return errors.WithMessage(err, string(m))
//     }
//
// This handler can then be used as follows:
//
//     func writeToGS(ctx context.Context, bucket, dst, src string) error {
//         return errd.Catch(func(e *errd.E) {
//             client, err := storage.NewClient(ctx)
//             e.Must(err, msg("error opening client"))
//             e.Defer(client)
//
//             w := client.Bucket(bucket).Object(dst).NewWriter(ctx)
//             e.Defer(w)
//
//             _, err = io.Copy(w, r)
//             e.Must(err, msg("error copying contents"))
//         })
//     }
//
// The storage package used in this example defines the errors that are
// typically a result of user error. It would be possible to write a more
// generic storage writer that will add additional clarification when possible.
// Using such a handler as a default handler would look like:
//
//     var ecGS = errd.New(DefaultHandler(...))
//
//     func writeToGS(ctx context.Context, bucket, dst, src string) error {
//         return ecGS.Run(func(e *errd.E) {
//             client, err := storage.NewClient(ctx)
//             e.Must(err)
//             e.Defer(client)
//             ...
//         })
//     }
//
// Setting up a global config with a default handler and using that everywhere
// makes it easy to enforce decorating errors. Error handlers can also be used
// to pass up HTTP error codes, log errors, attach metrics, etc.
//
//
// Returning Values
//
// A function that is passed to Run does have any return values, not even an
// error. Users are supposed to set return values in the outer scope, for
// example by using named return variables.
//
//    func foo() (name string, err error) {
//        return name, Run(func(e *errd.E) {
//            //   Some fun code here.
//            //   Set name at the end.
//            name = "bar"
//        })
//    }
//
// If name is only set at the end, any error will force an early return from Run
// and leave name empty. Otherwise, name will be set and err will be nil.
package errd

// TODO
//
// - Examples
//
// - Add semantic versioning.
//
// Consider these API changes:
//
// - Option to chose error, where a newly encountered error may be returned
//   instead of a previously selected one. Could be an option Replace in State.
// - Allow functions of signature func(), func(error), func() error, and
//   func(error) error, to be passed to Defer.
//
// For v2, if performance allows::
// - make E an interface: this allows injecting with test simulator.
//   Would be great for such functionality. Godocs will not look as nice,
//   though.
