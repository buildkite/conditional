package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/buildkite/conditional/internal/conformance"
)

func TestRunConformanceLocalOnly(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := runConformance(nil, &stdout, &stderr); err != nil {
		t.Fatalf("runConformance returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "local conformance cases passed") {
		t.Fatalf("stdout = %q, want local conformance summary", stdout.String())
	}
	if !strings.Contains(stdout.String(), "no server oracle configured") {
		t.Fatalf("stdout = %q, want oracle configuration hint", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunConformanceList(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := runConformance([]string{"--list"}, &stdout, &stderr); err != nil {
		t.Fatalf("runConformance returned error: %v", err)
	}

	dec := json.NewDecoder(&stdout)
	count := 0
	for {
		var request conformance.OracleRequest
		if err := dec.Decode(&request); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("decode request %d: %v", count, err)
		}
		if request.Name == "" || request.Source == "" || request.Expression == "" {
			t.Fatalf("request %d missing identifying fields: %#v", count, request)
		}
		count++
	}
	if count != len(conformance.Cases()) {
		t.Fatalf("listed %d cases, want %d", count, len(conformance.Cases()))
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunConformanceReportsOracleMismatches(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := runConformance([]string{"--oracle-command", `printf '{"result":false}\n'`}, &stdout, &stderr)
	if err == nil {
		t.Fatal("runConformance returned nil error, want mismatch")
	}
	if !strings.Contains(err.Error(), "server oracle mismatches") {
		t.Fatalf("error = %v, want mismatch summary", err)
	}
	if !strings.Contains(stderr.String(), "mismatch") {
		t.Fatalf("stderr = %q, want mismatch details", stderr.String())
	}
}

func TestRunOracleDecodesResult(t *testing.T) {
	result, err := runOracle(`printf '{"result":true}\n'`, 0, conformance.OracleRequest{})
	if err != nil {
		t.Fatalf("runOracle returned error: %v", err)
	}
	if result.Result == nil || !*result.Result {
		t.Fatalf("result = %#v, want true", result)
	}
}

func TestRunOracleReportsCommandFailure(t *testing.T) {
	_, err := runOracle(`printf 'nope' >&2; exit 7`, 0, conformance.OracleRequest{})
	if err == nil {
		t.Fatal("runOracle returned nil error, want command failure")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Fatalf("error = %v, want stderr context", err)
	}
}

func TestRunOracleReportsBadJSON(t *testing.T) {
	_, err := runOracle(`printf 'not-json\n'`, 0, conformance.OracleRequest{})
	if err == nil {
		t.Fatal("runOracle returned nil error, want decode failure")
	}
	if !strings.Contains(err.Error(), "decode oracle response") {
		t.Fatalf("error = %v, want decode context", err)
	}
}

func TestRunConformanceRejectsExtraArgs(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := runConformance([]string{"extra"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("runConformance returned nil error, want unexpected argument error")
	}
	if !strings.Contains(err.Error(), "unexpected arguments") {
		t.Fatalf("error = %v, want unexpected arguments", err)
	}
}

func TestRunOracleReportsTimeout(t *testing.T) {
	_, err := runOracle(`sleep 1`, 1, conformance.OracleRequest{})
	if err == nil {
		t.Fatal("runOracle returned nil error, want timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
}
