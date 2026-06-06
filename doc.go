// Package conditional validates and evaluates Buildkite conditional
// expressions.
//
// The public API is the root package. Use Context to provide Buildkite values,
// set Context.EntryPoint to the place where the conditional runs, then call
// Validate or Evaluate.
//
// Validate always returns parse and validation errors. Evaluate returns errors
// for build condition entrypoints. Notification entrypoints model Buildkite
// notification delivery, so Evaluate converts parse, validation, and evaluation
// errors to false for those entrypoints.
package conditional
