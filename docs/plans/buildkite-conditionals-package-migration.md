---
status: proposed
last_reviewed: 2026-06-06
related_plans:
  - docs/plans/buildkite-conditionals-parity.md
---

# Conditional Package Migration

The supported public API is the root package:

```go
import "github.com/buildkite/conditional"
```

The existing implementation packages remain available during the transition, but
they are not the final parity contract. Once the root API can evaluate the
server grammar directly, move implementation packages under `internal/`.

| Current package | Final home | Notes |
| --- | --- | --- |
| `ast` | `internal/ast` | Syntax model for the server grammar. |
| `lexer` | `internal/lexer` | Tokenizer for the server grammar. |
| `parser` | `internal/parser` | Parser plus source positions. |
| `evaluator` | `internal/evaluator` | Type-checked evaluator internals. |
| `object` | `internal/object` or replaced | Runtime values and type information. |
| `token` | `internal/token` | Parser implementation detail. |
| `repl` | `internal/repl` or removed | CLI/debugging helper, not library API. |
| `cmd/conditional` | `cmd/conditional` | CLI can depend on the root API. |

Do not add new external callers to implementation packages while the parity work
is in progress.
