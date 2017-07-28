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
// This code shows that error handling can be subtle and not the mindless
// check-decorate-return pattern it often seems to be. Does a Closer also
// support CloseWithError? Which panics need to be handled? Also note the
// subtle use of the return variable to convey an error in Copy to the last
// defer function.
// In practice, it is not unlikely that one would forget to handle the reader
// panicing, or even returning the error from Close,
// until this is discovered as a bug.
//
// Package errd aims to greatly reduce bugs resulting from such subtleties by
// making the default of having very strict error checking easy.
// The following code achieves the same as the above:
//
//     func writeToGS(ctx context.Context, bucket, dst, src string) error {
//         return errd.Run(func(e *errd.E) {
//             client, err := storage.NewClient(ctx)
//             e.Must(err)
//             e.DeferClose(client, errd.Discard)
//
//             w := client.Bucket(bucket).Object(dst).NewWriter(ctx)
//             e.DeferCloseWithError(w)
//
//             _, err = io.Copy(w, r)
//             e.Must(err)
//         })
//     }
//
// The `Discard` handler is passed to mimic the behavior of the original code
// to ignore any error resulting from closing the client.
//
// Package `errd` also allows automatic selection of the defer method using the
// `Defer` and `MustDefer` method, where the later is a shorthand of calling
// `Must` and `Defer` in sequence.
//
//     func writeToGS(ctx context.Context, bucket, dst, src string) error {
//         return errd.Run(func(e *errd.E) {
//             client, err := storage.NewClient(ctx)
//             e.MustDefer(client, err)
//
//             w := client.Bucket(bucket).Object(dst).NewWriter(ctx)
//             e.Defer(w)
//
//             _, err = io.Copy(w, r)
//             e.Must(err)
//         })
//     }
//
// Using these implicit defers will ensure the most conservative defer method
// is picked by default, simplifying understanding the code without consulting
// the docs.
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
// Deferring
//
// Package errd includes predefined defer handlers for unlocking sync.Lockers
// and closing io.Closers and Closers that include a CloseWithError method.
// Users can define their own by passing a DeferFunc to the DeferFunc method.
//
// With AutoDefer or Auto, errd will automatically select the defer handler
// based on the type of the value. In case of Closing, this has the advantage
// that one will not mistakenly forget to use CloseWithError when appropriate.
// By default errd will select the most conservative error passing strategy.
// Users may provide support for additional types with the DeferHandler Option.
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
