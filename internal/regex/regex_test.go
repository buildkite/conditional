package regex

import "testing"

func TestCompileAppliesFlagsAndTimeout(t *testing.T) {
	compiled, err := Compile(`\[skip tests\]`, "i")
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}
	if compiled.MatchTimeout != MatchTimeout {
		t.Fatalf("MatchTimeout = %v, want %v", compiled.MatchTimeout, MatchTimeout)
	}
	matched, err := compiled.MatchString("[SKIP TESTS]")
	if err != nil {
		t.Fatalf("MatchString returned error: %v", err)
	}
	if !matched {
		t.Fatal("case-insensitive regexp did not match")
	}
}

func TestCompileRejectsUnsupportedFlags(t *testing.T) {
	if _, err := Compile("main", "x"); err == nil {
		t.Fatal("Compile accepted unsupported regexp flag")
	}
}

func TestValidateRejectsServerUnsupportedFeatures(t *testing.T) {
	tests := []string{
		`(?<=a)b`,
		`(?<!a)b`,
		`(?>a*)a`,
		`a?+`,
		`a*+`,
		`a++`,
		`a{1,3}+`,
		`(?<name>group)`,
		`(?P<name>group)`,
		`(?'name'group)`,
		`(?(1)a|b)`,
	}

	for _, pattern := range tests {
		t.Run(pattern, func(t *testing.T) {
			if err := Validate(pattern); err == nil {
				t.Fatal("Validate accepted unsupported regexp feature")
			}
		})
	}
}

func TestValidateAllowsEscapedUnsupportedSyntax(t *testing.T) {
	if err := Validate(`\(\?<=a\)`); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}
