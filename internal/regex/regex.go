package regex

import (
	"fmt"
	"time"

	"github.com/dlclark/regexp2"
)

// MatchTimeout bounds regexp2 backtracking for conditional regex matches.
const MatchTimeout = time.Second

// Compile validates and compiles a server-compatible conditional regexp.
func Compile(pattern string, flags string) (*regexp2.Regexp, error) {
	options, err := options(flags)
	if err != nil {
		return nil, err
	}
	if err := Validate(pattern); err != nil {
		return nil, err
	}

	r, err := regexp2.Compile(pattern, options)
	if err != nil {
		return nil, fmt.Errorf("could not parse regexp: %v", err)
	}
	// regexp2 is intentionally used for Buildkite server-side syntax parity.
	// It can backtrack, so keep matching bounded.
	r.MatchTimeout = MatchTimeout
	return r, nil
}

func options(flags string) (regexp2.RegexOptions, error) {
	options := regexp2.RegexOptions(regexp2.RE2)
	for _, flag := range flags {
		switch flag {
		case 'i':
			options |= regexp2.IgnoreCase
		default:
			return regexp2.None, fmt.Errorf("unsupported regexp flag: %c", flag)
		}
	}

	return options, nil
}

// Validate rejects regexp features unsupported by the Buildkite server
// conditional regexp validator.
func Validate(pattern string) error {
	escaped := false
	inClass := false
	classFirst := false

	for i := 0; i < len(pattern); i++ {
		ch := pattern[i]
		if escaped {
			escaped = false
			if inClass {
				classFirst = false
			}
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if inClass {
			if ch == '[' && i+1 < len(pattern) && isPOSIXClassMarker(pattern[i+1]) {
				if end := classSetEnd(pattern, i); end != -1 {
					i = end
					classFirst = false
				}
				continue
			}
			if ch == ']' {
				if classFirst {
					classFirst = false
					continue
				}
				inClass = false
				continue
			}
			classFirst = false
			continue
		}
		if ch == '[' {
			inClass = true
			classFirst = true
			continue
		}

		if ch == '(' && i+1 < len(pattern) && pattern[i+1] == '?' {
			if hasPrefix(pattern, i, "(?#") {
				end := commentEnd(pattern, i+3)
				if end == -1 {
					return nil
				}
				i = end
				continue
			}
			switch {
			case hasPrefix(pattern, i, "(?<="):
				return unsupportedFeature("lookbehind")
			case hasPrefix(pattern, i, "(?<!"):
				return unsupportedFeature("nlookbehind")
			case hasPrefix(pattern, i, "(?>"):
				return unsupportedFeature("atomic")
			case hasPrefix(pattern, i, "(?<"):
				return unsupportedFeature("named_ab")
			case hasPrefix(pattern, i, "(?P<"):
				return unsupportedFeature("named_ab")
			case hasPrefix(pattern, i, "(?'"):
				return unsupportedFeature("named_sq")
			case hasPrefix(pattern, i, "(?("):
				return unsupportedFeature("condition_open")
			}
		}

		if i+1 < len(pattern) && pattern[i+1] == '+' {
			switch ch {
			case '?':
				return unsupportedFeature("zero_or_one_possessive")
			case '*':
				return unsupportedFeature("zero_or_more_possessive")
			case '+':
				return unsupportedFeature("one_or_more_possessive")
			case '}':
				if isBoundedQuantifierEnd(pattern, i) {
					return unsupportedFeature("bounded_possessive")
				}
			}
		}
	}

	return nil
}

func isPOSIXClassMarker(ch byte) bool {
	return ch == ':' || ch == '.' || ch == '='
}

func classSetEnd(pattern string, start int) int {
	if start+1 >= len(pattern) {
		return -1
	}
	marker := pattern[start+1]
	for i := start + 2; i+1 < len(pattern); i++ {
		if pattern[i] == marker && pattern[i+1] == ']' {
			return i + 1
		}
	}
	return -1
}

func commentEnd(pattern string, start int) int {
	for i := start; i < len(pattern); i++ {
		if pattern[i] == ')' {
			return i
		}
	}
	return -1
}

func isBoundedQuantifierEnd(pattern string, close int) bool {
	for open := close - 1; open >= 0; open-- {
		if pattern[open] != '{' {
			continue
		}
		if isEscapedByte(pattern, open) {
			return false
		}
		return isQuantifierBounds(pattern[open+1 : close])
	}
	return false
}

func isEscapedByte(pattern string, offset int) bool {
	backslashes := 0
	for i := offset - 1; i >= 0 && pattern[i] == '\\'; i-- {
		backslashes++
	}
	return backslashes%2 == 1
}

func isQuantifierBounds(bounds string) bool {
	if bounds == "" {
		return false
	}

	digitsBeforeComma := 0
	seenComma := false
	for i := 0; i < len(bounds); i++ {
		ch := bounds[i]
		switch {
		case ch >= '0' && ch <= '9':
			if !seenComma {
				digitsBeforeComma++
			}
		case ch == ',' && !seenComma:
			seenComma = true
		default:
			return false
		}
	}
	return digitsBeforeComma != 0
}

func hasPrefix(pattern string, offset int, prefix string) bool {
	return len(pattern)-offset >= len(prefix) && pattern[offset:offset+len(prefix)] == prefix
}

func unsupportedFeature(feature string) error {
	return fmt.Errorf("unsupported regexp feature: %s", feature)
}
