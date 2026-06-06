package evaluator

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/buildkite/conditional/object"
)

var shellIntegerPattern = regexp.MustCompile(`^[0-9]+$`)

type EnvScope interface {
	LookupEnv(key string) (string, bool)
}

func evalShellExpansion(raw string, scope Scope) object.Object {
	env, ok := scope.(EnvScope)
	if !ok {
		return newError("shell expansion evaluation requires environment scope")
	}

	value, set, err := evalShellRaw(raw, env)
	if err != nil {
		return newError("%s", err.Error())
	}
	if !set {
		return NULL
	}
	return &object.String{Value: value}
}

func evalStringLiteral(value string, quote string, scope Scope) object.Object {
	if quote != `"` || !ContainsShellTemplate(value) {
		return &object.String{Value: value}
	}

	env, ok := scope.(EnvScope)
	if !ok {
		return &object.String{Value: value}
	}

	out, err := evalShellString(value, env)
	if err != nil {
		return newError("%s", err.Error())
	}
	return &object.String{Value: out}
}

// ContainsShellTemplate reports whether a double-quoted string can produce a
// different runtime value after shell-style expansion.
func ContainsShellTemplate(value string) bool {
	return strings.Contains(value, "$$") || ContainsShellExpansion(value)
}

// ContainsShellExpansion reports whether a string contains a shell expression
// whose value depends on runtime environment.
func ContainsShellExpansion(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] != '$' {
			continue
		}
		if i+1 < len(value) && value[i+1] == '$' {
			i++
			continue
		}
		_, _, ok := readShellExpansion(value, i)
		if ok {
			return true
		}
	}
	return false
}

func evalShellRaw(raw string, env EnvScope) (string, bool, error) {
	if strings.HasPrefix(raw, "${") && strings.HasSuffix(raw, "}") {
		return evalBracedShell(raw[2:len(raw)-1], env)
	}
	if strings.HasPrefix(raw, "$") {
		name, rest, ok := splitShellName(raw[1:])
		if !ok {
			return "", false, fmt.Errorf("invalid shell expansion: %s", raw)
		}
		value, ok := env.LookupEnv(name)
		if rest != "" {
			if !ok {
				value = ""
			}
			return value + rest, true, nil
		}
		return value, ok, nil
	}
	return "", false, fmt.Errorf("invalid shell expansion: %s", raw)
}

func evalBracedShell(inner string, env EnvScope) (string, bool, error) {
	name, rest, ok := splitShellName(inner)
	if !ok {
		return "", false, fmt.Errorf("invalid shell expansion: ${%s}", inner)
	}

	if rest == "" {
		value, ok := env.LookupEnv(name)
		return value, ok, nil
	}

	if strings.HasPrefix(rest, ":") && !strings.HasPrefix(rest, ":-") && !strings.HasPrefix(rest, ":+") && !strings.HasPrefix(rest, ":?") {
		return evalShellSubstring(name, rest[1:], env)
	}

	operator, valueRaw, ok := splitShellOperator(rest)
	if !ok {
		return "", false, fmt.Errorf("invalid shell expansion: ${%s}", inner)
	}

	base, set := env.LookupEnv(name)
	op := operator
	if strings.HasPrefix(op, ":") {
		op = op[1:]
		if set && base == "" {
			set = false
		}
	}

	switch op {
	case "?":
		if set {
			return base, true, nil
		}
		return "", false, fmt.Errorf("parameter null or not set: %s", name)
	case "+":
		if !set {
			return "", true, nil
		}
		value, err := evalShellString(valueRaw, env)
		return value, true, err
	case "-":
		if set {
			return base, true, nil
		}
		value, err := evalShellString(valueRaw, env)
		return value, true, err
	default:
		return "", false, fmt.Errorf("invalid shell operator: %s", operator)
	}
}

func evalShellSubstring(name string, raw string, env EnvScope) (string, bool, error) {
	parts := splitTopLevel(raw, ':')
	if len(parts) < 1 || len(parts) > 2 {
		return "", false, fmt.Errorf("invalid shell substring: %s", raw)
	}

	x, err := evalShellInteger(parts[0], env)
	if err != nil {
		return "", false, err
	}

	base, ok := env.LookupEnv(name)
	if !ok {
		return "", false, nil
	}

	runes := []rune(base)
	if x >= len(runes) {
		return "", true, nil
	}
	end := len(runes)
	if len(parts) == 2 {
		y, err := evalShellInteger(parts[1], env)
		if err != nil {
			return "", false, err
		}
		end = x + y
	}
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[x:end]), true, nil
}

func evalShellInteger(raw string, env EnvScope) (int, error) {
	value, err := evalShellString(raw, env)
	if err != nil {
		return 0, err
	}
	if !shellIntegerPattern.MatchString(value) {
		return 0, fmt.Errorf("substring operation requires integer argument, not %q", value)
	}
	return strconv.Atoi(value)
}

func evalShellString(raw string, env EnvScope) (string, error) {
	var out strings.Builder
	for i := 0; i < len(raw); {
		switch raw[i] {
		case '$':
			if i+1 < len(raw) && raw[i+1] == '$' {
				out.WriteByte('$')
				i += 2
				continue
			}
			expansion, next, ok := readShellExpansion(raw, i)
			if !ok {
				out.WriteByte('$')
				i++
				continue
			}
			value, set, err := evalShellRaw(expansion, env)
			if err != nil {
				return "", err
			}
			if set {
				out.WriteString(value)
			}
			i = next
		case '\\':
			next := i
			for next < len(raw) && raw[next] == '\\' {
				next++
			}
			if next < len(raw) && raw[next] == '$' && next == i+1 {
				out.WriteByte('$')
				i = next + 1
				continue
			}
			out.WriteString(raw[i:next])
			i = next
		default:
			out.WriteByte(raw[i])
			i++
		}
	}
	return out.String(), nil
}

func splitShellName(inner string) (name string, rest string, ok bool) {
	if inner == "" || !isShellIdentStart(inner[0]) {
		return "", "", false
	}
	i := 1
	for i < len(inner) && isShellIdentPart(inner[i]) {
		i++
	}
	return inner[:i], inner[i:], true
}

func splitShellOperator(rest string) (operator string, value string, ok bool) {
	for _, op := range []string{":?", ":+", ":-", "?", "+", "-"} {
		if strings.HasPrefix(rest, op) {
			return op, rest[len(op):], true
		}
	}
	return "", "", false
}

func splitTopLevel(raw string, separator byte) []string {
	parts := []string{}
	start := 0
	depth := 0
	for i := 0; i < len(raw); i++ {
		if raw[i] == '$' && i+1 < len(raw) && raw[i+1] == '{' {
			depth++
			i++
			continue
		}
		if raw[i] == '}' && depth > 0 {
			depth--
			continue
		}
		if raw[i] == separator && depth == 0 {
			parts = append(parts, raw[start:i])
			start = i + 1
		}
	}
	parts = append(parts, raw[start:])
	return parts
}

func readShellExpansion(raw string, start int) (string, int, bool) {
	if start+1 >= len(raw) {
		return "", start, false
	}
	if raw[start+1] == '$' {
		return "$$", start + 2, false
	}
	if isShellIdentStart(raw[start+1]) {
		i := start + 2
		for i < len(raw) && isShellIdentPart(raw[i]) {
			i++
		}
		return raw[start:i], i, true
	}
	if raw[start+1] != '{' {
		return "", start, false
	}

	depth := 1
	for i := start + 2; i < len(raw); i++ {
		if raw[i] == '$' && i+1 < len(raw) && raw[i+1] == '{' {
			depth++
			i++
			continue
		}
		if raw[i] == '}' {
			depth--
			if depth == 0 {
				return raw[start : i+1], i + 1, true
			}
		}
	}
	return "", start, false
}

func isShellIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isShellIdentPart(ch byte) bool {
	return isShellIdentStart(ch) || (ch >= '0' && ch <= '9')
}
