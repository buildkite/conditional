package evaluator

import (
	"github.com/buildkite/conditional/internal/object"
	"github.com/buildkite/conditional/internal/shell"
)

type EnvScope interface {
	LookupEnv(key string) (string, bool)
}

func evalShellExpansion(raw string, scope Scope) object.Object {
	env, ok := scope.(EnvScope)
	if !ok {
		return newError("shell expansion evaluation requires environment scope")
	}

	value, set, err := shell.EvalRaw(raw, env)
	if err != nil {
		return newError("%s", err.Error())
	}
	if !set {
		return NULL
	}
	return &object.String{Value: value}
}

func evalStringLiteral(value string, raw string, quote string, scope Scope) object.Object {
	if raw == "" {
		raw = value
	}
	if quote != `"` || !ContainsShellTemplate(raw) {
		return &object.String{Value: value}
	}

	env, ok := scope.(EnvScope)
	if !ok {
		return &object.String{Value: value}
	}

	out, err := shell.EvalString(raw, env)
	if err != nil {
		return newError("%s", err.Error())
	}
	return &object.String{Value: out}
}

// ContainsShellTemplate reports whether a double-quoted string can produce a
// different runtime value after shell-style expansion.
func ContainsShellTemplate(value string) bool {
	return shell.ContainsTemplate(value)
}

// ContainsShellExpansion reports whether a string contains a shell expression
// whose value depends on runtime environment.
func ContainsShellExpansion(value string) bool {
	return shell.ContainsExpansion(value)
}

func evalShellString(raw string, env EnvScope) (string, error) {
	return shell.EvalString(raw, env)
}
