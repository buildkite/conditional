// Package conditional validates and evaluates Buildkite conditional
// expressions.
//
// The public API is the root package. Use Context to provide Buildkite values,
// set Context.EntryPoint to the place where the conditional runs, then call
// Validate or Evaluate.
//
// Build condition entrypoints return validation and evaluation errors to the
// caller. Notification entrypoints model Buildkite notification delivery and
// convert parse, validation, and evaluation errors to false.
package conditional
