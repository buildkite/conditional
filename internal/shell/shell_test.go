package shell

import "testing"

type env map[string]string

func (e env) LookupEnv(key string) (string, bool) {
	value, ok := e[key]
	return value, ok
}

func TestEvalRawPreservesBareSuffix(t *testing.T) {
	got, set, err := EvalRaw("$branch.deploy", env{"branch": "main"})
	if err != nil {
		t.Fatalf("EvalRaw returned error: %v", err)
	}
	if !set {
		t.Fatal("EvalRaw returned set=false")
	}
	if got != "main.deploy" {
		t.Fatalf("EvalRaw = %q, want %q", got, "main.deploy")
	}
}

func TestEvalStringEscapesUseBytes(t *testing.T) {
	got, err := EvalString(`\xff\377`, env{})
	if err != nil {
		t.Fatalf("EvalString returned error: %v", err)
	}

	want := string([]byte{0xff, 0xff})
	if got != want {
		t.Fatalf("EvalString bytes = %v, want %v", []byte(got), []byte(want))
	}
}

func TestEvalStringRejectsOutOfRangeOctal(t *testing.T) {
	if _, err := EvalString(`\400`, env{}); err == nil {
		t.Fatal("EvalString did not reject out-of-range octal escape")
	}
}

func TestReadExpansionIncludesNestedFallback(t *testing.T) {
	got, next, ok := ReadExpansion(`${branch:${empty:-1}:${two+2}}`, 0)
	if !ok {
		t.Fatal("ReadExpansion returned ok=false")
	}
	if got != `${branch:${empty:-1}:${two+2}}` {
		t.Fatalf("ReadExpansion token = %q", got)
	}
	if next != len(got) {
		t.Fatalf("ReadExpansion next = %d, want %d", next, len(got))
	}
}
