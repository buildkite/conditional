package conformance

import "testing"

func TestCasesMatchLocalEvaluator(t *testing.T) {
	for _, c := range Cases() {
		t.Run(c.Name, func(t *testing.T) {
			if c.Source == "" {
				t.Fatal("case source is empty")
			}
			if c.Expression == "" {
				t.Fatal("case expression is empty")
			}

			expected := Expected(c)
			actual := EvaluateLocal(c)
			if err := Compare(c.Mode, expected, actual); err != nil {
				t.Fatalf("local result mismatch: %v", err)
			}
		})
	}
}
