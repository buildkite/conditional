---
status: active
last_reviewed: 2026-06-06
spec_refs:
  - https://buildkite.com/docs/pipelines/configure/conditionals
---

# Buildkite Conditionals Parity

## Summary

This plan brings `buildkite/conditional` to exact parity with the server-side
Buildkite conditional language. The public docs remain the user-facing contract,
but the implementation contract is the server grammar, type checker, evaluator,
regular-expression validator, Buildkite context builder, and upstream specs in
`buildkite/buildkite`.

The current repository is now a modern Go module with `mise` tasks, CI wiring,
core boolean syntax, `null`, `includes`, short-circuiting logical operators, and
regexp2-based regular expressions. The remaining work is not just adding
operators. The library needs to become a polished Go package whose public API
evaluates Buildkite conditionals exactly as the server does, and whose internal
parser/evaluator packages are clean implementation details rather than a second
language with compatible-looking syntax.

The target outcome is a small, well-tested library that can answer the same
question as Buildkite: given a conditional expression, a Buildkite evaluation
context, and the source string form that Buildkite would evaluate, does the
expression evaluate to `true`, `false`, or a fail-closed error?

The hard part is parity evidence. The public docs list syntax, variables, and
examples, but some semantics are only observable from the server evaluator:
missing values, context-specific variables, type mismatches, dotted identifier
behavior, shell-style environment substitution, and server-rejected regular
expression features. The upstream `buildkite/buildkite` repo already has RSpec
coverage for much of this behavior. This plan therefore treats ported
table-driven Go conformance tests and an optional server oracle as first-class
deliverables, not test polish after implementation.

## Problem

The library currently evaluates a generic expression language with a generic
`object.Struct` scope. That is useful scaffolding, but it is not the same thing
as the server-side Buildkite conditional evaluator.

The current repo can parse and evaluate many documented examples:

- Comparators: `==`, `!=`, `=~`, `!~`.
- Logical operators: `||`, `&&`.
- Literals: integers, strings, booleans, and `null`.
- Parentheses, `!`, dotted object lookup, arrays, and `includes`.
- Regex literals, escaped `/`, `i` flags, RE2 compatibility mode, and bounded
  regexp2 matching.

Some of that current behavior is a useful foundation; some is divergent from
the server and should be removed from the Buildkite surface. The important gaps
are:

- The server grammar treats dotted names as flat identifiers. `build.env` is a
  function name and `build.branch` is an assigned variable name. The local
  object-lookup model is not enough for exact parity.
- The server parser supports ternary expressions and shell-style environment
  substitution forms such as `$branch`, `${branch:-fallback}`, and substring
  operations. Those are currently missing locally.
- The docs define many variables under `build`, `pipeline`, `organization`, and
  notification-only `step`, but this repo has no Buildkite-specific context
  builder or availability rules.
- The docs distinguish pipeline-level, step-level, build notification, and step
  notification conditionals. The current evaluator has no context kind, so it
  cannot reject or null-fill variables based on where the conditional runs.
- Missing documented nullable values need Buildkite-compatible behavior. Today a
  missing property is an error, while many documented variables should be `null`
  in specific contexts.
- The library has no stable root package API that consistently handles parser
  errors, evaluator errors, validation errors, and non-boolean final results.
- The local regex engine currently accepts some syntax the server rejects. Exact
  parity requires the Go library to reject server-rejected regex features even
  when regexp2 can evaluate them.
- The existing `@>` operator and any other non-server syntax should be removed
  from the Buildkite language surface instead of preserved as compatibility
  extensions.
- There is no local server-derived conformance corpus yet. The upstream
  `buildkite/buildkite` specs should be ported before inventing bespoke edge
  cases, otherwise each PR risks matching the docs examples but drifting from
  production behavior.

## Goals

- Match the server-side Buildkite conditional grammar, including syntax that is
  covered by upstream server specs but not emphasized in the public docs.
- Match variable availability, type checking, enum validation, nullable values,
  and evaluation behavior for every server context this library supports.
- Make `env()` and `build.env()` work with server-compatible validation and
  return semantics.
- Preserve fail-closed behavior: parse errors, unsupported syntax, unsupported
  flags, non-boolean final values, and context-ineligible variables must not
  silently evaluate to `true`.
- Add durable table-driven conformance tests that include every docs example
  plus upstream Buildkite server cases.
- Publish a small, idiomatic root Go package API for validation and evaluation.
- Keep implementation packages cohesive and testable, with clear
  single-responsibility boundaries and small interfaces only at consuming
  boundaries.
- Keep CI simple: `mise run check` should remain the default validation path.

## Non-Goals

- Do not model scheduler behavior, plugin execution, group-step expansion,
  dynamic pipeline uploads, branch filtering, or dependency behavior. The docs
  discuss those topics, but this repo should own expression parsing and
  evaluation, not pipeline scheduling.
- Do not implement Buildkite pipeline YAML parsing. The conditional parser
  should implement the server conditional grammar, including the shell-style
  substitution syntax that grammar accepts, but it should not become a YAML
  loader.
- Do not make live Buildkite API calls in default unit tests. Server oracle tests
  should be optional and clearly separated from deterministic local tests.
- Do not preserve divergent syntax for compatibility in the Buildkite evaluator.
  If a feature is not accepted by the server-side conditional language, remove
  it or keep it behind a clearly separate non-Buildkite internal test path until
  it can be deleted.

## Target Model

Make the module root, `github.com/buildkite/conditional`, the polished public
library surface. The root package should expose the Buildkite contract in terms
callers care about: validate this conditional, evaluate it in this Buildkite
context, and receive a boolean or a typed error.

Public shape:

```go
type EntryPoint string

const (
	EntryPointBuildCondition         EntryPoint = "build_condition"
	EntryPointBuildConditionWithStep EntryPoint = "build_condition_with_step"
	EntryPointBuildNotification      EntryPoint = "build_notification"
	EntryPointStepNotification       EntryPoint = "step_notification"
)

type Context struct {
	EntryPoint EntryPoint

	Build        Build
	Pipeline     Pipeline
	Organization Organization
	Step         *Step

	// BuildEnv is build-scoped env. ProjectEnv is pipeline/project env.
	// Merging these maps produces the server's Build::PipelineEnvironment.
	BuildEnv   map[string]string
	ProjectEnv map[string]string
}

func Validate(expression string, ctx Context) error
func Evaluate(expression string, ctx Context) (bool, error)
```

The API should do the server work callers should not have to remember:

- Parse the server grammar, including comments, ternaries, regex literals,
  flat dotted identifiers, and shell-style environment substitutions.
- Type-check the expression against the selected Buildkite context.
- Build the server-compatible assignment table for documented variables and
  functions.
- Evaluate to a final boolean and fail closed for every parser, validation,
  evaluation, regex, and non-boolean result error.

The entrypoints should mirror the reachable server paths:

- `EntryPointBuildCondition` matches `Build::Condition.evaluate`,
  `Build::Condition.validate`, and `Build::Condition.context` without a step.
- `EntryPointBuildConditionWithStep` matches the same server path with the
  optional `step:` argument or validator `{ step: true }`.
- `EntryPointBuildNotification` matches `Build::Notification#deliverable?`,
  including false-on-parse/evaluation-error behavior.
- `EntryPointStepNotification` matches `Step::Notification#deliverable?`,
  including step variables and false-on-parse/evaluation-error behavior.

Blank-string validation is a validator behavior, not an evaluator behavior:
`Validate("", ctx)` should match the upstream validator's accepted blank case,
while `Evaluate("", ctx)` should fail closed unless upstream evidence shows the
server evaluates blank conditionals directly.

Public context data should be explicit enough to preserve null versus empty
values. Nullable scalar server values should use pointers, maps should preserve
absent versus empty strings, and nil slices should represent server `null` where
the server distinguishes `nil` from an empty array. The initial public data
model should include at least:

```go
type Build struct {
	ID           *string
	State        *string
	Fixed        *bool
	BlockedState *string
	Source       *string
	SourceEvent  *string
	SourceAction *string
	Branch       *string
	Tag          *string
	Message      *string
	Commit       *string
	Number       *int

	Creator       Actor
	Author        Actor
	SCM           SCM
	PullRequest   PullRequest
	MergeQueue    MergeQueue
	TriggeredFrom TriggeredFrom
	RebuiltFrom   RebuiltFrom
}

type Actor struct {
	ID       *string
	Name     *string
	Email    *string
	Teams    []string
	Verified *bool
}

type Pipeline struct {
	ID                                    *string
	Name                                  *string
	Slug                                  *string
	DefaultBranch                         *string
	Repository                            *string
	StartedPassing                        *bool
	StartedFailing                        *bool
	NextFinishedBuildExists               *bool
	UseMergeQueueBaseCommitForGitDiffBase *bool
}

type SCM struct {
	AuthorName     *string
	AuthorEmail    *string
	CommitterName  *string
	CommitterEmail *string
}

type PullRequest struct {
	ID                *string
	BaseBranch        *string
	Draft             *bool
	Label             *string
	Labels            []string
	Repository        *string
	RepositoryFork    *bool
	UsingMergeRefspec *bool
}

type MergeQueue struct {
	Active     bool
	BaseBranch *string
	BaseCommit *string
}

type TriggeredFrom struct {
	BuildID      *string
	BuildNumber  *int
	PipelineSlug *string
	JobID        *string
}

type RebuiltFrom struct {
	BuildID     *string
	BuildNumber *int
}

type Organization struct {
	ID   *string
	Slug *string
}

type Step struct {
	ID      *string
	Key     *string
	Type    *string
	Label   *string
	State   *string
	Outcome *string
}
```

Fields such as visible teams and organization-preferred creator emails are
caller-supplied database-backed facts. Preferred creator email resolution should
populate `Actor.Email`, matching the server's `build.creator.email` assignment
rather than inventing a separate conditional variable. Pure conditional values
are derived in-library when the server does so, such as `build.source_event`
from `Build.SourceEvent` or `BUILDKITE_GITHUB_EVENT` when
`Build.Source == "webhook"`.

Some public context fields exist only to derive supported built-in environment
values. For example, `Pipeline.Name` feeds
`build.env("BUILDKITE_PIPELINE_NAME")`, but `pipeline.name` is not a server
conditional variable and must fail validation.

Enum validation follows the server enum definitions when the public docs differ.
For example, `pipeline.started_failing` is a boolean conditional variable, but
`started_failing` is not a `Build::State` value in `Build::Condition`.

Environment merge behavior is part of the public contract:

- `ProjectEnv` and `BuildEnv` are merged using server-compatible precedence.
  The exact precedence must be sourced from `Build::PipelineEnvironment` and
  captured in tests before implementing the merge.
- A present empty string remains distinct from an absent key.
- `env("NAME")` returns the merged value as a string, so absent values become
  `""` when the server does that.
- `build.env("NAME")` returns `null` for absent values and `""` for present
  empty strings.

Implementation packages should have clear responsibilities:

- `lexer` tokenizes the server grammar.
- `parser` parses tokens into AST nodes and preserves source positions.
- `ast` models syntax, not evaluation policy.
- `evaluator` evaluates a type-checked AST against a server-style context.
- `object` or its replacement represents runtime values and type information.
- A Buildkite context package owns variable/function assignment construction,
  enum definitions, and context availability.

Codebase cleanup should push toward idiomatic Go package boundaries:

- Prefer concrete types in the public API. Introduce interfaces only where a
  consumer needs substitution, such as an optional server-oracle checker.
- Keep parser, evaluator, regex validation, context construction, and
  environment substitution as separate reasons to change.
- Move implementation-only packages under `internal/` as part of the breaking
  cleanup. The root package is the supported library API.
- Remove or isolate any generic-language behavior that conflicts with the
  server grammar.

## Server Sources To Match

Treat these upstream files as the parity contract when they differ from local
behavior:

- `buildkite/buildkite:app/models/conditional/grammar.kpeg`
  defines parser syntax. Important implications: dotted variable and function
  names are flat identifiers; `build.env("NAME")` is a function named
  `build.env`, not a method call through an object receiver; ternary `? :` and
  shell-style environment substitutions are server grammar.
- `buildkite/buildkite:app/models/conditional/regexp.rb`
  defines accepted regex flags and rejected regex features. The Go
  implementation may use regexp2, but it must reject syntax the server rejects.
- `buildkite/buildkite:app/models/conditional/type_check_visitor.rb`,
  `evaluation_visitor.rb`, `variable.rb`, `function.rb`, `enum.rb`, and
  `context.rb` define type checking, evaluation, functions, enums, nullable
  values, and assignment lookup.
- `buildkite/buildkite:app/models/build/condition.rb`
  defines the Buildkite assignment table, `env()` versus `build.env()`,
  Buildkite enum values, context construction, and nullable build data.
- The upstream specs under `spec/models/conditional`,
  `spec/models/build/condition_spec.rb`, notification specs, and
  `spec/validators/build_condition_validator_spec.rb` are the primary source of
  conformance cases to port.

## Upstream Spec Port Manifest

Maintain a manifest while porting upstream specs. Each upstream spec group must
end in exactly one state: `ported`, `blocked`, `intentionally_excluded`, or
`superseded`. A slice should not claim parity for a feature until every relevant
upstream group is accounted for.

Initial manifest:

| Upstream source | Groups to account for | Current status | Required status before parity claim |
| --- | --- | --- | --- |
| `spec/models/conditional/parser_spec.rb` | friendly errors, comments, objects/properties, function calls, complex strings, simple expressions, operand precedence, negation, token positions | `blocked`: Slice 3 ports flat dotted identifiers/functions, ternary precedence, shell expansion operands, malformed dotted-name rejection, and parser-level rejection of `@>`; Slice 4 ports server string escape decoding for single-quoted and double-quoted strings plus quote-aware shell fallback scanning. Generic custom test functions are superseded by concrete Buildkite functions in the root API. Exact friendly messages and token positions remain follow-up parser work | `ported` or `intentionally_excluded` with reason |
| `spec/models/conditional/evaluator_spec.rb` | booleans, nulls, arrays, regexes, string comparisons, ternaries, variables/enums, shell substitutions | `blocked`: Slice 4 ports regex includes, null regex/includes semantics, shell substitution evaluation, double-quoted interpolation, server string escapes, shell fallback string grammar, enum literal validation, logical type-checking before runtime short-circuiting, enum comparison asymmetry, array equality/null comparisons, Ruby-style falsey runtime behavior for typed nil booleans, and representative type-checker rejection cases. Generic custom test functions are superseded by the root API's concrete `env` and `build.env` functions | `ported` |
| `spec/models/conditional/variable_spec.rb` | typed variables, nullable typed values, enums, lazy values | `blocked`: Slice 4 ports the server's declared-type behavior for nil values: typed nil strings, booleans, arrays, and enums type-check as their declared type rather than nullable unions. The Ruby lazy-variable API is superseded by Go context fields and derived helpers; exhaustive field coverage remains Slice 5 work | `ported` or `superseded` by Go type/context tests |
| `spec/models/build/condition_spec.rb` | `env()`, `build.env()`, build/pipeline/org fields, webhook fields, pull request label, project env merge, validation, context construction | `blocked`: Slice 2 seeds source-tagged root cases for representative `env()`, `build.env()`, organization, pipeline, webhook, pull request label, project env merge, and validation behavior; Slice 5 adds server-style static env validation for typos, unsupported built-ins, invalid names, and names starting with `$`, plus full-data, nil-data, SCM, merge queue, visible-team, empty-team, verified-creator, and preferred-email context matrix coverage through the public API; remaining context work is exhaustive entrypoint availability and any server cases not yet accounted for | `ported` |
| `spec/validators/build_condition_validator_spec.rb` | blank/nil validation, invalid conditionals, step-variable validation option | `blocked`: Slice 2 ports blank string, invalid expression, step-variable rejection, and step-option acceptance through root validation; nil validation is not representable in the string API | `ported` or `intentionally_excluded` with reason |
| `spec/models/build/notification_spec.rb` | no conditional, false on unmet condition, false on parser/evaluation errors | `blocked`: Slice 2 ports blank/no-condition equivalent, false-on-unmet condition, false-on-parse-error, and false-on-unavailable-step-variable behavior; full notification parsing/config propagation remains out of scope | `ported` |
| `spec/models/step/notification_spec.rb` | step variables and false-on-error notification behavior | `blocked`: Slice 2 ports step key/id checks and false-on-parse-error through the step notification entrypoint; full step notification model behavior remains out of scope | `ported` |
| `spec/models/build/pipeline_config/build_notifications_spec.rb` | config parsing and notification conditional propagation | `blocked`: config parsing is not in the current slice | `blocked` until config parsing is in scope, or `intentionally_excluded` with reason |
| `spec/models/build/pipeline_config/step_notifications_spec.rb` | config parsing and step notification conditional propagation | `blocked`: config parsing is not in the current slice | `blocked` until config parsing is in scope, or `intentionally_excluded` with reason |

The manifest can live in this plan while work is small. If it becomes too large,
move it to `docs/plans/buildkite-conditionals-upstream-manifest.md` and link it
from this plan.

## Current State

### Landed

- Modern Go structure is in place: `go.mod`, `cmd/conditional`, `mise.toml`, and
  `.buildkite/pipeline.yml`.
- `mise run check` runs `go vet`, `go build`, `go test`, and `staticcheck`.
- Core docs syntax is mostly implemented:
  - `==`, `!=`, `=~`, `!~`
  - `||`, `&&`
  - integers, strings, booleans, `null`
  - parentheses and `!`
  - `includes`
  - arrays
  - comments
  - regex literals with escaped `/`, `i` flags, regexp2, RE2 compatibility mode,
    and match timeout
- Parser rejects trailing tokens after a complete expression.
- Lexer handles unterminated regex literals without panicking.
- README documents the current regex interpolation boundary.
- Slice 1 landed the root package API, server-derived entrypoint model, public
  context structs, typed error categories, root smoke tests, and package
  migration map.
- Slice 2 landed the source-tagged, table-driven root conformance suite. The
  suite keeps test data in Go code, requires every root conformance case to name
  its docs or upstream `buildkite/buildkite` source, and splits behavior across
  syntax, evaluation, regex, context, and root API error test files.
- Slice 3 landed parser grammar alignment for flat dotted identifiers/functions,
  ternary parsing, shell expansion operands, parser-level `@>` rejection, and
  unterminated string/substitution errors.
- Slice 4 now aligns the root type checker with server declared-type semantics:
  Buildkite assignments keep their declared type even when the runtime value is
  nil, enums are not interchangeable with strings except for server-supported
  enum-to-static-string comparisons, logical operands are type-checked on both
  sides before runtime short-circuiting, and ternary branches do not create
  flow-sensitive nullable unions.
- Slice 4 also ports server string escape semantics for single-quoted,
  double-quoted, and shell fallback strings. String tokens now keep both decoded
  values and raw source bodies so escaped dollars stay static while unescaped
  shell substitutions still evaluate through the Buildkite environment.
- Runtime `env()` and `build.env()` calls now enforce the server's blank-name
  and `BUILDKITE_*` allowlist checks after interpolation, so dynamic names fail
  closed the same way static literal names do.
- Some landed behavior is still explicitly provisional because it diverges from
  the server regex validator or still lacks the exhaustive Buildkite context
  matrix.

### Active Slice

- Slice 5 is now filling out the Buildkite context and `env()` matrix after the
  Slice 4 public-surface audit. Generic Ruby custom functions and lazy-variable
  wrappers are not a public Go API; concrete Buildkite functions, typed context
  fields, and derived helpers are the parity target.
- Slice 4 aligned type checking and evaluator semantics with the server
  evaluator for the public Buildkite surface:
- Root `Validate` now uses server-style type checking instead of evaluating the
  expression just to prove it returns a boolean. Unknown variables/functions,
  wrong arity, incompatible operators, invalid `env()` argument types, enum
  mismatches, and non-boolean root results fail before runtime.
- Evaluator semantics now match upstream cases for regex `includes`, `null
  includes ...`, `null =~ /.../`, `null !~ /.../`, and short-circuiting branches
  that would otherwise fail at runtime.
- Runtime evaluation now uses the server's Ruby truthiness for logical and
  ternary decisions, so typed nil booleans are falsey and `!nil` is true through
  the root API.
- Equality semantics now allow array-vs-array and array-vs-null comparisons,
  while `includes` rejects enum values as enum token types rather than treating
  them as plain strings.
- Shell expansion operands now evaluate against the merged Buildkite
  environment for set, unset, empty, required, default, alternate, substring,
  nested substring argument, and bad substring length cases.
- Double-quoted strings now evaluate shell substitutions, collapse `$$` to a
  literal dollar, and decode the server escape set. Single-quoted strings decode
  only the server-supported `\\` and `\'` escapes and keep shell-looking text
  literal.
- Shell fallback strings now decode server escapes, support nested single- and
  double-quoted fallback strings, and keep braces inside nested fallback quotes
  from closing the surrounding `${...}` expression.
- `env()` and `build.env()` now reject blank dynamic names and unsupported
  dynamic `BUILDKITE_*` names during evaluation. Notification entrypoints still
  convert those evaluation errors to `false`, matching the server deliverability
  paths.
- The built-in environment matrix now covers triggered-from build/job values,
  rebuilt-from values, pull request merge-refspec state, and merge-queue
  `BUILDKITE_GIT_DIFF_BASE` behavior, including the server's blank-string
  defaults for supported built-in keys.
- Server-supported cases that the current implementation cannot pass are not
  added as skipped tests. They remain recorded in the manifest and known gaps
  until the parser, evaluator, context, and regex parity slices implement them.
- `docs/plans/buildkite-conditionals-package-migration.md` records the intended
  movement from public implementation packages to `internal/` once the root API
  is ready to own the parity contract.

### Known Gaps

| Area | Current Behavior | Required Direction |
| --- | --- | --- |
| Dotted names | Parser and evaluator internals now use flat dotted identifiers for server variables. Some public implementation packages still expose older generic-language concepts during transition. | Finish removing nested object lookup assumptions and move implementation packages under `internal/` in the cleanup slice. |
| `build.env()` | `build.env("NAME")` parses as a flat function identifier, type-checks with the server's string return token type, evaluates to `null` for absent variables, and fails closed for blank or unsupported dynamic `BUILDKITE_*` names. | Expand validator and conformance coverage for the full server env matrix in Slice 5. |
| Ternary syntax | Ternaries parse with server precedence, evaluate lazily, use Ruby truthiness for nil runtime conditions, and type-check branch compatibility without local nullable-union narrowing. | Expand conformance coverage for every upstream ternary type-checker case before marking Slice 4 complete. |
| Shell substitution | Shell expansion operands, double-quoted interpolation, server string escapes, and quoted fallback strings evaluate for the upstream set/unset/empty/default/alternate/required/substring matrix and representative fallback grammar cases. | Run a final upstream parser/evaluator audit before marking every shell substitution group accounted for. |
| Scope | Callers pass arbitrary `object.Struct`. | Add server-style Buildkite assignment tables with documented variables and context availability. |
| Nullable values | Documented nullable Buildkite assignments are present as runtime `null` while keeping their server-declared type for validation. Truly unknown variables still fail closed. | Finish the exhaustive context matrix and lazy variable coverage so every documented nullable field is covered in every entrypoint. |
| Context restrictions | Root entrypoints now model build conditions, build conditions with a step, build notifications, and step notifications. `step.*` fails validation unless the entrypoint supplies a step, and notification entrypoints convert parse, validation, and evaluation errors to `false`. | Finish auditing entrypoint-specific docs/server differences, especially variables documented as notification-only but exposed by `Build::Condition.context`. |
| Final result | Root `Validate`/`Evaluate` now type-check for a boolean final result; lower-level `Eval` still returns any `object.Object` during transition. | Move implementation packages under `internal/` and keep root `(bool, error)` as the supported Buildkite surface. |
| Regex syntax | regexp2 accepts some features the server rejects. | Keep regexp2 only with a server-compatible validator for flags and unsupported constructs. |
| Divergent operators | `@>` no longer tokenizes as a Buildkite parser operator. Some lower-level compatibility constants may remain until cleanup. | Remove remaining dead `@>` constants or generic-language artifacts in the cleanup slice. |
| Type mismatch semantics | Core equality, regex matching, `includes`, `!`, logical, ternary, enum, null, array comparison, and concrete Buildkite function cases now use server-derived type-checking behavior. | Finish exact error category coverage and the remaining context-driven function cases. |
| Conformance | Root package tests are now split into source-tagged table-driven files for syntax, evaluation, regex, context, and root API error behavior. The tables seed docs and upstream spec coverage but do not yet port every blocked upstream group. | Expand the tables in each implementation slice until every manifest group is `ported`, `superseded`, or `intentionally_excluded` before claiming parity. |

## Server Syntax And Context Surface To Cover

The public docs define these expression features, and they must all be covered:

- Comparators: `==`, `!=`, `=~`, `!~`.
- Logical operators: `||`, `&&`.
- Array operator: `includes`.
- Integers, strings, `true`, `false`, `null`.
- Parentheses and `!`.
- Regular expressions, including RHS-only regex matching and escaping `$` anchors
  in pipeline YAML to avoid interpolation.
- `//` comments.

The upstream server grammar and specs add syntax that is also in scope for exact
server parity:

- Ternary expressions: `condition ? true_value : false_value`.
- Dotted variable identifiers as flat assignment names, such as `build.branch`,
  `pipeline.slug`, and `organization.slug`.
- Dotted function identifiers as flat function names, such as `build.env`.
- Shell-style environment substitution in expressions and double-quoted strings:
  `$name`, `${name}`, `${name?}`, `${name:?}`, `${name-default}`,
  `${name:-default}`, `${name+alternate}`, `${name:+alternate}`, and substring
  forms such as `${name:1:2}`.
- Server string escape behavior for single-quoted strings, double-quoted
  strings, and substitution fallback strings.
- Server type checking for strings, numbers, booleans, arrays, nulls, regexes,
  typed variables, functions, and enums.

The docs and upstream `Build::Condition` context define this variable and
function surface:

- Common build variables: author, branch, commit, creator, id, message, number,
  pull request data, merge queue data, source, source event/action, state, tag.
  Author values are unverified webhook data; creator team values depend on
  Buildkite being able to identify a verified user.
- `build.env("NAME")`, including documented `BUILDKITE_*` variables and caller
  supplied custom variables.
- Pipeline variables: default branch, id, repository, slug, started failing, and
  started passing.
- Organization variables: id and slug.
- Step notification variables: id, key, label, type, state, and outcome.

The plan should not assume all variables are valid in all contexts. The docs
explicitly call out context-specific behavior, such as `build.state` for
notification-level conditionals and `step.*` for step notifications.

The public `EntryPoint` values should come from server entrypoints, not from
pipeline concepts invented in this repo:

- `Build::Condition` without `step` means build, pipeline, organization, and
  build env assignments are available; `step.*` is invalid.
- `Build::Condition` with `step` or validator `{ step: true }` adds `step.*`.
- `Build::Notification#deliverable?` uses build condition evaluation and returns
  false instead of surfacing parser/evaluation errors.
- `Step::Notification#deliverable?` uses build condition evaluation with step
  assignments and returns false instead of surfacing parser/evaluation errors.

## Go Design Constraints

The final codebase should be a small Go library, not a transliteration of the
Ruby object model. Apply SOLID principles in idiomatic Go terms:

- Single responsibility: lexer, parser, regex validation, type checking,
  evaluation, environment substitution, and Buildkite context construction each
  get one reason to change.
- Open/closed: add new server variables, functions, and enum values through
  explicit assignment/type definitions and conformance tests, not by widening
  the evaluator to accept arbitrary unknown names.
- Liskov/interface substitution: avoid broad exported interfaces. Where
  substitution matters, such as an optional server oracle, define the smallest
  consumer-owned interface.
- Interface segregation: public callers should not need lexer/parser/evaluator
  internals to evaluate a conditional.
- Dependency inversion: the public evaluator depends on a Buildkite context
  contract; optional live server checks depend on an oracle interface, not on
  hard-coded network calls in unit tests.

Additional Go constraints:

- Exported identifiers in the root package need doc comments and stable error
  behavior.
- Error tests should use typed errors or error categories, plus source location
  where meaningful. Do not couple the Go suite to every word of a Ruby error
  unless exact message parity becomes a requirement.
- Test helper packages should stay small. Prefer local helper functions with
  `t.Helper()` over a custom testing framework.
- Do not introduce abstractions just to mirror Ruby classes. Add them only when
  they remove real duplication or protect the public API from implementation
  churn.

## Delivery Strategy

### Slice 1: Public API, Context Model, And Manifest Foundation

Create the root package API and public context model before the broader
conformance suite. This slice gives later tests a stable surface to compile
against and removes the risk of designing tests around internal packages.

Definition of done:

- `EntryPoint`, `Context`, `Build`, `Pipeline`, `Organization`, `Step`, and
  nested context structs exist in the root package with doc comments.
- `Validate(expression, ctx)` and `Evaluate(expression, ctx)` exist in the root
  package and fail closed through typed error categories.
- EntryPoint behavior is derived from upstream server paths:
  `Build::Condition` without step, `Build::Condition` with step,
  `Build::Notification#deliverable?`, and `Step::Notification#deliverable?`.
- The root API can represent absent versus empty string values, nil versus empty
  arrays, build env versus project env, webhook source event/action inputs,
  teams, preferred emails, and nullable fields.
- The exact `BuildEnv`/`ProjectEnv` merge order is documented from
  `Build::PipelineEnvironment`, not inferred from public docs.
- Existing subpackages are either moved under `internal/` or a package migration
  map is committed in this slice showing the exact follow-up move. The final
  state remains internal implementation packages plus root public API.
- The upstream spec port manifest is committed with statuses for every upstream
  spec group listed above.
- Add a small smoke test set through the root API for a passing expression, a
  parser error, a non-boolean result, and a notification false-on-error path.

### Slice 2: Idiomatic Go Conformance Test Suite

Create the test shape before more feature work. Keep the test data in Go code so
cases are easy to read, refactor, and debug with normal `go test` output. Do not
add YAML files as test data.

Proposed files:

- Root package behavior tests:
  - `conditional_test.go`
  - `syntax_test.go`
  - `eval_test.go`
  - `context_test.go`
  - `regex_test.go`
- Package-local tests should remain where they are useful for narrow parser,
  lexer, or evaluator failures.
- A tiny test helper file is acceptable if it removes real duplication. Avoid a
  custom test framework.

Table shape:

```go
tests := []struct {
	name       string
	source     string
	expression string
	ctx        Context
	want       bool
	wantError  errorKind
}{
	{
		name:       "docs branch starts with features slash",
		source:     "docs/pipelines/configure/conditionals",
		expression: `build.branch =~ /^features\//`,
		ctx: Context{
			EntryPoint: EntryPointBuildCondition,
			Build:      Build{Branch: str("features/foo")},
		},
		want: true,
	},
	{
		name:       "missing tag is null",
		source:     "buildkite/buildkite spec/models/conditional/evaluator_spec.rb",
		expression: `build.tag == null`,
		ctx:        Context{EntryPoint: EntryPointBuildCondition},
		want: true,
	},
}
```

`str` in examples is a test helper that returns `*string`; production callers can
use pointers directly or helper constructors if the final API provides them.

Upstream sources to port:

- `buildkite/buildkite:spec/models/conditional/parser_spec.rb`
  for comments, operator precedence, negation, complex strings, regex literals,
  arrays, `includes`, parser error categories, and token-position-sensitive
  syntax cases.
- `buildkite/buildkite:spec/models/conditional/evaluator_spec.rb`
  for boolean/null/string comparisons, arrays, regex matching, function calls,
  ternary conditionals, environment-variable interpolation, and enum behavior.
- `buildkite/buildkite:spec/models/conditional/variable_spec.rb`
  for typed variables, nullable typed values, enum validation, and lazy values.
- `buildkite/buildkite:spec/models/build/condition_spec.rb`
  for `env()`, `build.env()`, `organization.*`, `pipeline.*`, `build.*`,
  pull request label behavior, webhook source event/action behavior, project env
  merging, validation rules, and the server context builder.
- `buildkite/buildkite:spec/validators/build_condition_validator_spec.rb`
  for blank conditionals, invalid expressions, and step-variable availability
  when the validator is configured for step context.
- `buildkite/buildkite:spec/models/build/notification_spec.rb` and
  `buildkite/buildkite:spec/models/step/notification_spec.rb`
  for fail-closed notification behavior and `step.*` availability.
- `buildkite/buildkite:spec/models/build/pipeline_config/build_notifications_spec.rb`
  and
  `buildkite/buildkite:spec/models/build/pipeline_config/step_notifications_spec.rb`
  should be mined once notification parsing is in scope.

Porting order:

1. Direct expression semantics: comments, precedence, negation, booleans,
   nulls, strings, numbers, arrays, `includes`, escaped regex delimiters, regex
   flags, ternaries, and short-circuiting.
2. Shell substitution semantics from the server evaluator: set, unset, empty,
   default, alternate, required, substring, nested fallback, and bad substring
   length cases.
3. Negative parser, type-checker, and evaluator cases represented as stable Go
   error categories: wrong operators, unknown variables, invalid function calls,
   invalid regex literals, final non-boolean values, invalid `env()` arguments,
   enum mismatches, and maximum nesting.
4. Buildkite context cases:
   `organization.*`, `pipeline.*`, `build.*`, `env()`, `build.env()`, webhook
   event/action fields, pull request label fields, project env merging, step
   variables, notification contexts, nullable documented values, and enum
   validation.
5. Divergence tests: local-only syntax such as `@>` and regex features rejected
   by the server should have explicit rejection tests.

Definition of done:

- Every docs example expression is represented in table-driven tests.
- Ported upstream examples carry a `source` string that points back to the
  originating `buildkite/buildkite` spec file.
- Every upstream group in the manifest is marked `ported`, `blocked`,
  `intentionally_excluded`, or `superseded`; easy cases cannot be silently
  cherry-picked while hard groups disappear.
- Server-supported upstream cases are either passing tests or recorded in this
  plan as the next implementation slice. Do not land permanently skipped parity
  tests.
- Tests can assert parse errors, evaluation errors, and boolean results.
- Test helpers assert errors by kind and location where meaningful, not by
  brittle full-message string comparisons unless message parity is explicitly in
  scope.
- Existing unit tests still run through `mise run check`.
- The helper makes parser errors visible; tests must not evaluate a nil or
  erroneous AST by accident.

Current Slice 2 progress:

- `test_helpers_test.go` provides small pointer helpers and shared
  `runEvaluateCases` / `runValidateCases` helpers that require every case to
  name its source.
- `syntax_test.go`, `eval_test.go`, `regex_test.go`, `context_test.go`, and
  `conditional_test.go` split the root suite by behavior rather than by a large
  monolithic fixture.
- The docs operator reference is covered for comparators, logical operators,
  `includes`, integers, strings, booleans, `null`, parentheses, regex literals,
  prefix `!`, and comments.
- The docs example expressions are seeded for branch equality/inequality,
  feature branch regex, tag presence, tag regex via variable and `build.env()`,
  case-insensitive message regex, scheduled source, custom env, creator teams,
  draft pull requests, and merge queue base branch.
- Docs examples that encode YAML/env-substitution escaping for `$` anchors are
  recorded as blocked until Slice 3 implements shell-style substitution; the
  current regex tests separately assert raw `$` anchors and escaped literal
  dollars so the parser does not conflate those semantics.
- Representative upstream cases are ported from parser, evaluator,
  `Build::Condition`, build condition validator, build notification, and step
  notification specs. The manifest remains `blocked` for every upstream group
  that still needs feature work rather than skipped tests.

Status: landed.

### Slice 3: Parser Grammar Parity

Make the parser match `app/models/conditional/grammar.kpeg`.

Definition of done:

- Dotted variable identifiers parse as flat assigned-variable names.
- Dotted function identifiers parse as flat function names.
- Ternary expressions parse with server precedence.
- Shell substitution syntax parses in operands, double-quoted strings, fallback
  strings, and substring arguments.
- String escape behavior matches the server grammar.
- Regex literal parsing matches server delimiters and the optional `i` flag.
- `@>` and any other non-server operators are removed from the Buildkite
  grammar.
- Parser no-panic tests cover unterminated strings, regexes, comments,
  substitutions, and deeply nested expressions.

Current Slice 3 progress:

- Flat dotted identifiers and dotted function names are implemented.
- Ternary parsing and lazy evaluation are implemented.
- Shell expansion operands are tokenized and parsed, including nested brace
  forms used by substring expressions.
- Double-quoted string interpolation remains blocked until Slice 4's environment
  substitution evaluator. That evaluator needs to distinguish unset, null, and
  empty values and cannot be represented by the current `StringLiteral` alone.
- `@>` is rejected by the parser rather than by root validation.

Status: landed.

### Slice 4: Type Checking And Evaluation Semantics

Port the server type-checker and evaluator behavior without copying the Ruby
object model.

Definition of done:

- Equality, inequality, regex matching, `includes`, logical operators, `!`, and
  ternary evaluation match upstream specs.
- Arrays, nulls, booleans, strings, numbers, regexes, typed variables, functions,
  and enums are type-checked before evaluation.
- Unknown variables/functions, wrong arity, unsupported operators, incompatible
  comparisons, and non-boolean logical operands fail closed.
- Short-circuiting prevents skipped branches from evaluating missing variables
  or failing functions, matching server behavior.
- Environment substitution evaluates set, unset, empty, required, default,
  alternate, and substring forms the same way as the server.

Current Slice 4 progress:

- Root validation now runs a compact server-style type checker before
  evaluation. The checker covers documented Buildkite variables, the current
  `env` and `build.env` functions, operator compatibility, enum literal
  validation for the modeled Buildkite enums, final boolean result validation,
  and unknown variable/function rejection.
- Evaluation now matches upstream server cases for regex `includes`, null array
  includes, null regex matching/non-matching, and lazy short-circuiting of
  runtime-failing shell substitutions.
- Shell substitutions now evaluate for `$name`, `${name}`, `${name?}`,
  `${name:?}`, `${name-default}`, `${name:-default}`, `${name+alternate}`,
  `${name:+alternate}`, substring forms, nested substring arguments, unset
  values, empty values, and invalid substring lengths.
- Double-quoted strings interpolate shell substitutions and collapse `$$` to a
  literal dollar. Single-quoted strings remain literal.
- String escape evaluation now matches the server grammar for single-quoted,
  double-quoted, and shell fallback strings, including newline, space, control,
  byte-oriented hex and octal escapes, out-of-range octal rejection, unknown
  double-quoted escapes, single-quoted `\\`/`\'`, escaped dollars, and braces
  inside nested quoted fallback strings.
- Runtime environment function arguments now mirror the server after string
  interpolation: blank names and unsupported `BUILDKITE_*` names produce
  evaluation errors for build conditions and `false` for notification
  deliverability checks, while dynamic custom names remain runtime lookups.
- Remaining Slice 4 audit before marking the slice landed: a final pass over the
  upstream evaluator/parser groups to ensure no substitution grammar or public
  Buildkite type-checker cases are unaccounted for. Generic Ruby-only custom
  function and lazy-variable wrapper specs are superseded by the root Go API's
  concrete function and context model.

### Slice 5: Buildkite Context And `env()` Semantics

Implement `Build::Condition` context behavior in Go.

Definition of done:

- Context structs cover every server assignment in `Build::Condition`, including
  deprecated-but-server-supported values such as `build.fixed` if the server
  still exposes them.
- Nullable documented variables are materialized as `null` when valid but
  absent.
- Context-ineligible variables fail closed instead of being silently available.
- `env()` returns server-compatible strings and validation behavior.
- `build.env()` is a flat function identifier and returns server-compatible
  nullable values.
- `BUILDKITE_*` allowlist validation, typo suggestions, invalid names, and names
  starting with `$` match server error categories.
- Project env and build env merge semantics match `Build::PipelineEnvironment`.
- `step.*` only when the server path supplies a step: build condition
  validation/evaluation with a step and step notification conditionals.
- `build.pull_request.*` values on PR and non-PR builds.
- `build.merge_queue.*` values on merge queue and non-merge-queue builds.
- Verified and unverified actor cases for `build.author.*`,
  `build.creator.*`, `build.author.teams`, and `build.creator.teams`.
- Enumerated values for `build.source`, `build.state`, `step.type`,
  `step.state`, and `step.outcome`.
- `build.source_event`, `build.source_action`, and `build.pull_request.label`
  webhook behavior matches upstream tests.
- Verified-user-sensitive values and visible team lists are represented in the
  caller-provided context model without hard-coding server database behavior.

Current Slice 5 progress:

- Built-in `BUILDKITE_*` values are derived for branch, tag, message, commit,
  pipeline, organization, pull request, merge queue, triggered-from,
  rebuilt-from, pull request labels and merge-refspec state, and merge-queue
  git-diff-base fields.
- `pipeline.name` has been removed from the conditional variable surface
  because `Build::Condition` does not assign it; `Pipeline.Name` remains as
  caller-provided data for `BUILDKITE_PIPELINE_NAME`.
- `build.state` and `step.state` enum validation now tracks the server enum
  models for current drift: `build.state` rejects `started_failing`, while
  `step.state` accepts `waiting_for_input` and `canceled`.
- Static `env()` and `build.env()` validation now matches the server's error
  categories for malformed names, names starting with `$`, unsupported
  `BUILDKITE_*` variables, and close supported-variable typos.
- Source-tagged context tests now cover the upstream `.context` real-data and
  nil-data matrices for build, pipeline, organization, actor, SCM, pull request,
  and merge queue assignments. Actor coverage includes nil teams, empty teams,
  caller-supplied visible team slugs, verified creator state, and preferred
  email values.
- `build.pull_request.label` now follows the server helper exactly: it is gated
  by `build.source_event == "pull_request"` and otherwise exposes the
  caller-provided webhook payload label without also requiring a
  `build.source_action`.
- `BUILDKITE_GIT_DIFF_BASE` is gated by explicit merge-queue build state, then
  uses either the merge queue base branch or base commit according to the
  pipeline provider setting.
- Covered built-in environment keys whose server fallback is a blank string now
  materialize as `""` through `build.env()` instead of `null`, matching
  `Build::PipelineEnvironment#[]`.

### Slice 6: Regex Exact Parity

Keep regexp2 as the matcher, but validate regex syntax against the server's
accepted feature set.

Validator strategy:

1. Parse literal delimiters and flags in the conditional parser. Only no flag or
   `i` is accepted.
2. Run a dedicated regex validator before compiling with regexp2. The validator
   should reject explicit server-denied constructs from `Conditional::Regexp`:
   lookbehind, negative lookbehind, atomic groups, possessive quantifiers,
   named captures, and regex conditionals.
3. Compile with regexp2 only after validation, with the existing timeout.
4. Maintain an accepted/rejected regex matrix sourced from
   `spec/models/conditional/evaluator_spec.rb`,
   `app/models/conditional/regexp.rb`, and any additional upstream regex tests.
5. If Ruby `Regexp::Scanner` rejects a construct that cannot be classified with
   a simple lexical validator, add a focused parser/validator case and record
   the reason in the manifest before claiming regex parity.

Required coverage:

- RHS-only regex matching.
- Escaped `/` delimiters.
- `i` flag.
- Unsupported flags.
- Server-rejected constructs from `Conditional::Regexp`, including lookbehind,
  negative lookbehind, atomic groups, possessive quantifiers, named captures, and
  conditionals.
- Server validation for shorthand character classes, POSIX classes, and any
  scanner cases where Ruby rejects syntax that regexp2 would accept.
- Literal dollar matches: `/\$/`, `/fee\$/`.
- Raw anchor matches: `/fee$/`, `/^v[0-9]+\.0$/`.

Definition of done:

- Test names distinguish regex anchors and literal-dollar matches from
  shell-substitution cases.
- Regex timeout behavior has a focused test that cannot make CI slow or flaky.
- The library rejects server-rejected regex features even if regexp2 can execute
  them.

### Slice 7: Divergence Removal And Codebase Cleanup

Remove syntax, tests, examples, and package shape that conflict with exact
server parity.

Definition of done:

- `@>` is removed from the Buildkite parser/evaluator/tests/docs unless upstream
  server evidence proves it is accepted.
- README examples are valid server conditionals. For example, remove or fix the
  current `meta-data("foo")` example because the docs say build meta-data is not
  available in conditional expressions.
- Nested object lookup is not part of the Buildkite evaluation surface unless it
  is only an internal representation of flat assignments.
- Public docs describe the root package API, entrypoints, variable
  availability, supported syntax, and fail-closed behavior.
- Package boundaries are reviewed for cohesion, exported identifiers, comments,
  and unnecessary interfaces.

### Slice 8: Optional Server Oracle

Add a tool for checking table-driven conformance cases against Buildkite server
behavior.

Possible shape:

```sh
mise run conformance:check
```

The oracle should be optional because it may need Buildkite credentials, a test
pipeline, or private/internal access. Default CI should run committed Go tables
only.

Definition of done:

- The tool can evaluate the Go conformance cases against the server or a
  server-backed API and report mismatches.
- If recording observed server behavior is useful later, add generated output as
  a deliberate follow-up rather than as the default test data format.
- If a live oracle is not available, the plan records exactly which semantics
  remain inferred from docs rather than server-proven.

## Verification

Run on every implementation slice:

```sh
mise run check
git diff --check
```

Add these targeted checks as the plan lands:

- Root package table-driven tests for every supported server behavior.
- Package-local unit tests for lexer/parser/type-checker/evaluator behavior
  where failures are easier to diagnose below the public API.
- Ported conformance tests for docs examples and upstream server-derived edge
  cases.
- Fuzz tests or bounded randomized tests for lexer/parser no-panic behavior,
  especially regex literals, strings, comments, shell substitutions, ternaries,
  and nested expressions.
- Regression tests for short-circuiting so missing values in skipped branches do
  not fail evaluation.
- Regression tests for nullable documented variables versus unknown properties.
- Regex timeout test for a pathological regexp2 pattern, written with a short
  timeout and generous assertion so it is deterministic.
- Rejection tests for divergent local syntax and server-rejected regex features.
- `go test -run` examples in PR descriptions for narrow iteration, with
  `mise run check` as the final validation.
- Buildkite pipeline validation with `bk pipeline validate --file
  .buildkite/pipeline.yml` when CI config changes.

## Resolved Decisions

- The repo uses `master` as the default branch.
- `mise run check` is the canonical local validation command.
- Exact server-side Buildkite syntax and semantics are the target. Public docs
  are necessary coverage, but upstream server grammar/spec behavior is also in
  scope.
- Divergent local syntax can be removed. `@>` should not remain in the
  Buildkite language unless upstream server evidence proves it is accepted.
- Dotted names are flat server identifiers, not object/method syntax in the
  Buildkite grammar.
- Ternary expressions, shell-style environment substitutions, typed variables,
  enums, and dotted function names are required parity work because upstream
  server specs cover them.
- `regexp2` remains the regex engine, but the library must reject regex features
  the server rejects. The linear-time guarantee from Go's regexp engine is not a
  requirement, but regex matching must stay bounded by timeout.
- Keep regex literal semantics in the parser. Raw `$` is a regex anchor and
  `\$` is a literal dollar; shell substitution handling belongs to the server
  grammar, not heuristic regex rewriting.
- Port upstream Buildkite specs into plain table-driven Go tests before
  inventing bespoke parity cases. Do not use YAML test data for the conformance
  corpus.
- The root package is the supported Go library API. Existing subpackages can
  remain during transition, but they are not the final parity contract.
- Breaking API and package cleanup is allowed to reach the polished library
  shape, including moving implementation packages under `internal/`.
- Root-package `Validate(expression, ctx)` and `Evaluate(expression, ctx)` are
  the public API names.
- Public entrypoints are derived from reachable server paths:
  `Build::Condition` without step, `Build::Condition` with step,
  `Build::Notification#deliverable?`, and `Step::Notification#deliverable?`.
- Error parity means exact accept/reject behavior, stable Go error categories,
  and source location where meaningful. Byte-for-byte Ruby error text is not a
  requirement for the first parity release.
- The library should derive pure conditional values, such as `source_event` from
  supplied environment data, but callers must provide database-backed facts such
  as visible teams and preferred emails.
- Implementation packages should move under `internal/` once the root API is in
  place.
- Upstream spec porting requires a manifest with a status for every relevant
  spec group. A feature cannot claim parity from cherry-picked tests alone.
- Slice 1 is the root API/context/manifest foundation; the broader table-driven
  conformance suite starts in Slice 2 so it has a compiling public API to target.

## Deferred Work

- Optional live server oracle. Ported upstream specs should carry the first
  several implementation slices.
- Byte-for-byte error text if the first release can provide stable typed errors,
  source locations, and exact accept/reject behavior.

## Key Learnings From Pressure-Testing

- Exact parity cannot be proven from the public docs examples alone. The
  upstream grammar and specs define real server syntax, including ternary
  expressions and shell-style environment substitutions, that must be included.
- Dotted identifiers are a larger architectural issue than `build.env()` alone.
  The server uses flat assignment/function names; a nested object lookup model
  will keep producing edge-case drift.
- Regex `$` handling should not be solved with parser heuristics. Raw `$` and
  `\$` have normal regex meaning inside regex literals; shell substitution is a
  separate part of the server grammar.
- The current generic scope API makes simple tests easy but hides Buildkite's
  nullable, typed, enum, and context-specific variable rules. A server-style
  Buildkite context builder is the smallest useful abstraction for parity.
- Table-driven conformance tests should land before more behavior changes. They
  give every follow-up PR a durable place to encode docs examples, server
  observations, and regressions.
- Conformance tests still need a public API to compile against. The first slice
  now creates the root API, context model, and manifest foundation before the
  broader table suite lands.
- The upstream `buildkite/buildkite` specs provide the best available parity
  corpus. Porting those examples into Go tables should be the default source of
  new tests, with invented cases reserved for gaps the upstream specs and docs
  do not cover.
- "Port upstream specs" must be measurable. The manifest prevents easy upstream
  cases from masking unported hard groups.
- Regex parity cannot be delegated to regexp2. The plan now requires a dedicated
  server-compatible validator plus an accepted/rejected matrix.
- SOLID in this repo should mean cohesive Go package responsibilities and a
  small public root API. It should not mean adding broad interfaces or a Ruby-like
  class structure.
