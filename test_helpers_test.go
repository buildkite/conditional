package conditional

import "testing"

const (
	docsConditionalsSource = "https://buildkite.com/docs/pipelines/configure/conditionals"

	upstreamParserSpec             = "buildkite/buildkite:spec/models/conditional/parser_spec.rb"
	upstreamEvaluatorSpec          = "buildkite/buildkite:spec/models/conditional/evaluator_spec.rb"
	upstreamBuildConditionSpec     = "buildkite/buildkite:spec/models/build/condition_spec.rb"
	upstreamBuildValidatorSpec     = "buildkite/buildkite:spec/validators/build_condition_validator_spec.rb"
	upstreamBuildNotificationSpec  = "buildkite/buildkite:spec/models/build/notification_spec.rb"
	upstreamStepNotificationSpec   = "buildkite/buildkite:spec/models/step/notification_spec.rb"
	upstreamConditionalRegexpModel = "buildkite/buildkite:app/models/conditional/regexp.rb"
)

type evaluateCase struct {
	name       string
	source     string
	expression string
	ctx        Context
	want       bool
	wantError  ErrorKind
}

type validateCase struct {
	name       string
	source     string
	expression string
	ctx        Context
	wantError  ErrorKind
}

func runEvaluateCases(t *testing.T, tests []evaluateCase) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireCaseSource(t, tt.source)

			got, err := Evaluate(tt.expression, tt.ctx)
			if tt.wantError != "" {
				if !IsErrorKind(err, tt.wantError) {
					t.Fatalf("Evaluate(%q) error = %v, want %s", tt.expression, err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("Evaluate(%q) returned error: %v", tt.expression, err)
			}
			if got != tt.want {
				t.Fatalf("Evaluate(%q) = %t, want %t", tt.expression, got, tt.want)
			}
		})
	}
}

func runValidateCases(t *testing.T, tests []validateCase) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireCaseSource(t, tt.source)

			err := Validate(tt.expression, tt.ctx)
			if tt.wantError != "" {
				if !IsErrorKind(err, tt.wantError) {
					t.Fatalf("Validate(%q) error = %v, want %s", tt.expression, err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate(%q) returned error: %v", tt.expression, err)
			}
		})
	}
}

func requireCaseSource(t *testing.T, source string) {
	t.Helper()

	if source == "" {
		t.Fatal("test case is missing source")
	}
}

func str(value string) *string {
	return &value
}

func boolptr(value bool) *bool {
	return &value
}
