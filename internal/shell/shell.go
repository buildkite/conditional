package shell

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var integerPattern = regexp.MustCompile(`^-?[0-9]+$`)

// Env provides environment values for shell-style substitution.
type Env interface {
	LookupEnv(key string) (string, bool)
}

// ContainsTemplate reports whether a double-quoted string can produce a
// different runtime value after shell-style expansion.
func ContainsTemplate(value string) bool {
	return strings.Contains(value, "$$") || ContainsExpansion(value)
}

// ContainsExpansion reports whether a string contains a shell expression whose
// value depends on runtime environment.
func ContainsExpansion(value string) bool {
	for i := 0; i < len(value); {
		switch value[i] {
		case '$':
			if i+1 < len(value) && value[i+1] == '$' {
				i += 2
				continue
			}
			_, next, ok := ReadExpansion(value, i)
			if ok {
				return true
			}
			if next > i {
				i = next
				continue
			}
			i++
		case '\\':
			_, next, err := ReadStringEscape(value, i)
			if err != nil {
				i++
				continue
			}
			i = next
		default:
			i++
			continue
		}
	}
	return false
}

// EvalRaw evaluates a standalone shell expansion.
func EvalRaw(raw string, env Env) (string, bool, error) {
	if strings.HasPrefix(raw, "${") && strings.HasSuffix(raw, "}") {
		return evalBraced(raw[2:len(raw)-1], env)
	}
	if strings.HasPrefix(raw, "$") {
		name, rest, ok := splitName(raw[1:])
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

func evalBraced(inner string, env Env) (string, bool, error) {
	name, rest, ok := splitName(inner)
	if !ok {
		return "", false, fmt.Errorf("invalid shell expansion: ${%s}", inner)
	}

	if rest == "" {
		value, ok := env.LookupEnv(name)
		return value, ok, nil
	}

	if strings.HasPrefix(rest, ":") && !strings.HasPrefix(rest, ":-") && !strings.HasPrefix(rest, ":+") && !strings.HasPrefix(rest, ":?") {
		return evalSubstring(name, rest[1:], env)
	}

	operator, valueRaw, ok := splitOperator(rest)
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
		value, err := EvalString(valueRaw, env)
		return value, true, err
	case "-":
		if set {
			return base, true, nil
		}
		value, err := EvalString(valueRaw, env)
		return value, true, err
	default:
		return "", false, fmt.Errorf("invalid shell operator: %s", operator)
	}
}

func evalSubstring(name string, raw string, env Env) (string, bool, error) {
	parts := splitTopLevel(raw, ':')
	if len(parts) < 1 || len(parts) > 2 {
		return "", false, fmt.Errorf("invalid shell substring: %s", raw)
	}

	x, err := evalInteger(parts[0], env)
	if err != nil {
		return "", false, err
	}

	base, ok := env.LookupEnv(name)
	if !ok {
		return "", false, nil
	}

	runes := []rune(base)
	if x < 0 {
		x = len(runes) + x
	}
	if x < 0 {
		x = 0
	}
	if x >= len(runes) {
		return "", true, nil
	}
	end := len(runes)
	if len(parts) == 2 {
		y, err := evalInteger(parts[1], env)
		if err != nil {
			return "", false, err
		}
		if y < 0 {
			end = len(runes) + y
		} else {
			end = x + y
		}
	}
	if end > len(runes) {
		end = len(runes)
	}
	if end < x {
		end = x
	}
	return string(runes[x:end]), true, nil
}

func evalInteger(raw string, env Env) (int, error) {
	value, err := EvalString(raw, env)
	if err != nil {
		return 0, err
	}
	value = strings.TrimSpace(value)
	if !integerPattern.MatchString(value) {
		return 0, fmt.Errorf("substring operation requires integer argument, not %q", value)
	}
	return strconv.Atoi(value)
}

// EvalString evaluates shell substitutions inside a string.
func EvalString(raw string, env Env) (string, error) {
	var out strings.Builder
	for i := 0; i < len(raw); {
		switch raw[i] {
		case '"', '\'':
			value, next, err := evalQuotedString(raw, i, env)
			if err != nil {
				return "", err
			}
			out.WriteString(value)
			i = next
		case '$':
			if i+1 < len(raw) && raw[i+1] == '$' {
				out.WriteByte('$')
				i += 2
				continue
			}
			expansion, next, ok := ReadExpansion(raw, i)
			if !ok {
				out.WriteByte('$')
				i++
				continue
			}
			value, set, err := EvalRaw(expansion, env)
			if err != nil {
				return "", err
			}
			if set {
				out.WriteString(value)
			}
			i = next
		case '\\':
			value, next, err := ReadStringEscape(raw, i)
			if err != nil {
				return "", err
			}
			out.WriteString(value)
			i = next
		default:
			out.WriteByte(raw[i])
			i++
		}
	}
	return out.String(), nil
}

func evalQuotedString(raw string, start int, env Env) (string, int, error) {
	quote := raw[start]
	var out strings.Builder
	for i := start + 1; i < len(raw); {
		if raw[i] == quote {
			return out.String(), i + 1, nil
		}

		if quote == '\'' {
			if raw[i] == '\\' {
				value, next, err := ReadSingleQuotedEscape(raw, i)
				if err != nil {
					return "", start, err
				}
				out.WriteString(value)
				i = next
				continue
			}
			out.WriteByte(raw[i])
			i++
			continue
		}

		switch raw[i] {
		case '$':
			if i+1 < len(raw) && raw[i+1] == '$' {
				out.WriteByte('$')
				i += 2
				continue
			}
			expansion, next, ok := ReadExpansion(raw, i)
			if !ok {
				out.WriteByte('$')
				i++
				continue
			}
			value, set, err := EvalRaw(expansion, env)
			if err != nil {
				return "", start, err
			}
			if set {
				out.WriteString(value)
			}
			i = next
		case '\\':
			value, next, err := ReadStringEscape(raw, i)
			if err != nil {
				return "", start, err
			}
			out.WriteString(value)
			i = next
		default:
			out.WriteByte(raw[i])
			i++
		}
	}
	return "", start, fmt.Errorf("unterminated shell string")
}

// ReadSingleQuotedEscape reads a backslash escape in a single-quoted shell
// fallback string.
func ReadSingleQuotedEscape(raw string, start int) (string, int, error) {
	if start+1 >= len(raw) {
		return "", start, fmt.Errorf("unterminated shell string escape")
	}
	switch raw[start+1] {
	case '\\', '\'':
		return string(raw[start+1]), start + 2, nil
	default:
		return raw[start : start+2], start + 2, nil
	}
}

// ReadStringEscape reads a backslash escape in a double-quoted shell string.
func ReadStringEscape(raw string, start int) (string, int, error) {
	if start+1 >= len(raw) {
		return "", start, fmt.Errorf("unterminated shell string escape")
	}

	next := raw[start+1]
	switch next {
	case 'n':
		return "\n", start + 2, nil
	case 's':
		return " ", start + 2, nil
	case 'r':
		return "\r", start + 2, nil
	case 't':
		return "\t", start + 2, nil
	case 'v':
		return "\v", start + 2, nil
	case 'f':
		return "\f", start + 2, nil
	case 'b':
		return "\b", start + 2, nil
	case 'a':
		return "\a", start + 2, nil
	case 'e':
		return "\x1b", start + 2, nil
	case '\\', '"':
		return string(next), start + 2, nil
	case 'x':
		if start+3 < len(raw) && isHexDigit(raw[start+2]) && isHexDigit(raw[start+3]) {
			value, err := strconv.ParseInt(raw[start+2:start+4], 16, 32)
			if err != nil {
				return "", start, err
			}
			return string([]byte{byte(value)}), start + 4, nil
		}
		return "x", start + 2, nil
	default:
		if isOctalDigit(next) {
			end := start + 2
			for end < len(raw) && end < start+4 && isOctalDigit(raw[end]) {
				end++
			}
			value, err := strconv.ParseInt(raw[start+1:end], 8, 32)
			if err != nil {
				return "", start, err
			}
			if value > 0xff {
				return "", start, fmt.Errorf("octal escape out of range: \\%s", raw[start+1:end])
			}
			return string([]byte{byte(value)}), end, nil
		}
		return string(next), start + 2, nil
	}
}

func splitName(inner string) (name string, rest string, ok bool) {
	if inner == "" || !isIdentStart(inner[0]) {
		return "", "", false
	}
	i := 1
	for i < len(inner) && isIdentPart(inner[i]) {
		i++
	}
	return inner[:i], inner[i:], true
}

func splitOperator(rest string) (operator string, value string, ok bool) {
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
		if raw[i] == '\\' {
			i++
			continue
		}
		if raw[i] == '"' || raw[i] == '\'' {
			next, ok := skipQuoted(raw, i)
			if !ok {
				continue
			}
			i = next - 1
			continue
		}
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

// ReadExpansion reads one shell expansion starting at start.
func ReadExpansion(raw string, start int) (string, int, bool) {
	if start+1 >= len(raw) {
		return "", start, false
	}
	if raw[start+1] == '$' {
		return "$$", start + 2, false
	}
	if isIdentStart(raw[start+1]) {
		i := start + 2
		for i < len(raw) && isExpansionPart(raw[i]) {
			i++
		}
		return raw[start:i], i, true
	}
	if raw[start+1] != '{' {
		return "", start, false
	}

	depth := 1
	for i := start + 2; i < len(raw); i++ {
		if raw[i] == '\\' {
			i++
			continue
		}
		if raw[i] == '"' || raw[i] == '\'' {
			next, ok := skipQuoted(raw, i)
			if !ok {
				return "", start, false
			}
			i = next - 1
			continue
		}
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

func skipQuoted(raw string, start int) (int, bool) {
	quote := raw[start]
	escaped := false
	for i := start + 1; i < len(raw); i++ {
		if raw[i] == quote && !escaped {
			return i + 1, true
		}
		escaped = raw[i] == '\\' && !escaped
		if raw[i] != '\\' {
			escaped = false
		}
	}
	return start, false
}

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentPart(ch byte) bool {
	return isIdentStart(ch) || (ch >= '0' && ch <= '9')
}

func isExpansionPart(ch byte) bool {
	return isIdentPart(ch) || ch == '.'
}

func isOctalDigit(ch byte) bool {
	return ch >= '0' && ch <= '7'
}

func isHexDigit(ch byte) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}
