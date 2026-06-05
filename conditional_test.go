package conditional

import (
	"testing"
)

func TestEvaluateBuildCondition(t *testing.T) {
	branch := "features/api"

	got, err := Evaluate(`build.branch =~ /^features\//`, Context{
		EntryPoint: EntryPointBuildCondition,
		Build:      Build{Branch: &branch},
	})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if !got {
		t.Fatalf("Evaluate returned false, want true")
	}
}

func TestEvaluateDefaultsToBuildCondition(t *testing.T) {
	branch := "features/api"

	got, err := Evaluate(`build.branch == "features/api"`, Context{
		Build: Build{Branch: &branch},
	})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if !got {
		t.Fatalf("Evaluate returned false, want true")
	}
}

func TestEvaluateRejectsUnknownEntryPoint(t *testing.T) {
	_, err := Evaluate(`true`, Context{EntryPoint: "unknown"})
	if !IsErrorKind(err, ErrorKindValidation) {
		t.Fatalf("Evaluate error = %v, want %s", err, ErrorKindValidation)
	}
}

func TestEvaluateMergesProjectEnvBeforeBuildEnv(t *testing.T) {
	got, err := Evaluate(`env("DEPLOY_ENV") == "build" && env("EMPTY") == "" && env("MISSING") == ""`, Context{
		EntryPoint: EntryPointBuildCondition,
		ProjectEnv: map[string]string{
			"DEPLOY_ENV": "project",
			"EMPTY":      "",
		},
		BuildEnv: map[string]string{
			"DEPLOY_ENV": "build",
		},
	})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if !got {
		t.Fatalf("Evaluate returned false, want true")
	}
}

func TestEvaluateReturnsParseError(t *testing.T) {
	_, err := Evaluate(`nope != == one`, Context{EntryPoint: EntryPointBuildCondition})
	if !IsErrorKind(err, ErrorKindParse) {
		t.Fatalf("Evaluate error = %v, want %s", err, ErrorKindParse)
	}
}

func TestEvaluateReturnsResultErrorForNonBoolean(t *testing.T) {
	_, err := Evaluate(`"not boolean"`, Context{EntryPoint: EntryPointBuildCondition})
	if !IsErrorKind(err, ErrorKindResult) {
		t.Fatalf("Evaluate error = %v, want %s", err, ErrorKindResult)
	}
}

func TestNotificationEntryPointFailsClosed(t *testing.T) {
	got, err := Evaluate(`nope != == one`, Context{EntryPoint: EntryPointBuildNotification})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if got {
		t.Fatalf("Evaluate returned true, want false")
	}

	got, err = Evaluate(`step.key == "deploy"`, Context{EntryPoint: EntryPointBuildNotification})
	if err != nil {
		t.Fatalf("Evaluate returned error for unavailable step variable: %v", err)
	}
	if got {
		t.Fatalf("Evaluate returned true for unavailable step variable, want false")
	}
}

func TestValidateAcceptsBlank(t *testing.T) {
	if err := Validate("   ", Context{EntryPoint: EntryPointBuildCondition}); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestStepAvailabilityFollowsEntryPoint(t *testing.T) {
	key := "deploy"

	got, err := Evaluate(`step.key == "deploy"`, Context{
		EntryPoint: EntryPointBuildConditionWithStep,
		Step:       &Step{Key: &key},
	})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if !got {
		t.Fatalf("Evaluate returned false, want true")
	}

	err = Validate(`step.key == "deploy"`, Context{EntryPoint: EntryPointBuildCondition})
	if !IsErrorKind(err, ErrorKindValidation) {
		t.Fatalf("Validate error = %v, want %s", err, ErrorKindValidation)
	}

	_, err = Evaluate(`step.key == "deploy"`, Context{EntryPoint: EntryPointBuildCondition})
	if !IsErrorKind(err, ErrorKindValidation) {
		t.Fatalf("Evaluate error = %v, want %s", err, ErrorKindValidation)
	}
}
