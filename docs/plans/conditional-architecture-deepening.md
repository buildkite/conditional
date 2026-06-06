---
status: active
last_reviewed: 2026-06-06
related_plans:
  - docs/plans/buildkite-conditionals-parity.md
---

# Conditional architecture deepening

## Summary

The parity plan landed the public Buildkite conditional surface. This plan
keeps that behavior stable while reducing the remaining scaffolding from the
generic expression evaluator that existed before parity work.

The target outcome is the same public contract: callers use `Validate` and
`Evaluate` with a `Context`, and the library returns a boolean or typed
fail-closed error. The internal modules should now make server drift cheaper to
absorb by concentrating the Buildkite-specific rules that currently live in
parallel maps and helper paths.

## Problem

The current code is correct enough to pass the parity suite, but some modules
are still shallow:

- Buildkite assignments are duplicated between runtime values and declared
  validation types.
- Shell substitution is scanned in the lexer, interpreted by the evaluator, and
  inspected by type checking.
- Regex validation and regexp2 compilation live inside the parser.
- The command and object unmarshalling paths still preserve old generic
  evaluator behavior that is not part of the public Buildkite surface.

That split makes future server drift harder than it needs to be. A new server
variable, env value, shell grammar detail, or regex restriction should have one
obvious module to update.

## Goals

- Preserve the root package interface and all parity behavior.
- Deepen Buildkite context construction so assignment names, declared types,
  availability, and runtime values stay together.
- Deepen shell substitution so scanning, runtime detection, and evaluation share
  one implementation path.
- Move server regex validation and compilation out of parser control flow.
- Remove generic evaluator scaffolding that no longer earns its interface.

## Non-goals

- Do not add new public APIs.
- Do not change documented conditional semantics.
- Do not re-open the landed parity decisions.
- Do not add interfaces unless there are already multiple real adapters.
- Do not expand into pipeline YAML parsing or scheduler behavior.

## Invariants

- `Validate` and `Evaluate` remain the supported public seam.
- Root conformance tests stay source-tagged and continue to exercise the public
  Buildkite interface.
- Unknown variables, unsupported env names, invalid regex syntax, type
  mismatches, and non-boolean final results fail closed.
- `ProjectEnv`, `BuildEnv`, and built-in Buildkite env values keep the existing
  merge order and absent-versus-empty behavior.

## Current progress

- Slice 1 is implemented. Assignment definitions now drive both runtime
  assignments and declared validation types. `go test ./...` passes.
- Slice 2 is implemented. Lexer tokenization, runtime shell detection, and
  evaluator shell substitution now share `internal/shell`. Focused shell,
  lexer, evaluator, and full package tests pass.
- Slice 3 is implemented. Server regex validation, flag handling, regexp2
  compilation, and timeout assignment now live in `internal/regex`. Focused
  regex, parser, and full package tests pass.
- Slice 4 is implemented. The unused `internal/object.Unmarshal` path is gone,
  the REPL now uses the root package evaluator with a Buildkite context derived
  from process env, and old nested-scope benchmark fixtures have been flattened.
  `go test ./...` and `mise run check` pass.

## Delivery strategy

### Slice 1: Buildkite assignment definitions

Create one internal assignment definition model in the root package and use it
to build both runtime `object.Struct` values and declared validation types.
Move step assignment availability through the same path.

Definition of done:

- Adding or changing a conditional assignment no longer requires editing
  parallel assignment/type maps.
- Existing context, env, and declared-type tests pass unchanged.

### Slice 2: Shell substitution module

Extract shell substitution reading and escape handling so lexer token detection,
runtime string detection, and evaluator substitution share one parser for shell
templates.

Definition of done:

- Shell parsing behavior is concentrated in one module.
- Existing root shell substitution tests pass unchanged.

### Slice 3: Regex module

Move server regex feature validation, flag handling, regexp2 compilation, and
match timeout into a regex-focused module used by the parser.

Definition of done:

- Parser code no longer owns the regex feature matrix.
- Regex validation tests still prove server-compatible accepted and rejected
  forms.

### Slice 4: Generic evaluator cleanup

Remove obsolete generic scaffolding that is not used by the public Buildkite
surface. Update or remove the command path if it cannot honor root package
validation and evaluation.

Definition of done:

- `internal/object.Unmarshal` and its tests are gone if no production caller
  needs them.
- The command path either uses the root package seam or is removed from the
  supported build.
- Internal evaluator tests focus on implementation behavior that remains useful
  after the root conformance suite.

## Verification

- `go test ./...`
- `mise run check`
- Focused package tests during each slice when the touched module has a narrow
  package boundary.

## Key learnings from pressure-testing

- A new public interface would work against the cleanup goal. These slices
  should deepen existing modules behind the root seam.
- The root conformance suite is the safest regression detector. Package-local
  tests should support narrow implementation behavior, not preserve a second
  generic language.
- The landed parity plan is still the source of truth for semantics; this plan
  changes locality and leverage, not the Buildkite contract.
