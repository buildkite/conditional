package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/buildkite/conditional/internal/conformance"
)

func runConformance(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("conformance", flag.ContinueOnError)
	flags.SetOutput(stderr)

	list := flags.Bool("list", false, "write conformance cases as JSON lines")
	oracleCommand := flags.String("oracle-command", os.Getenv("CONDITIONAL_ORACLE_COMMAND"), "external server oracle command")
	oracleTimeout := flags.Duration("oracle-timeout", 30*time.Second, "timeout for each oracle case")

	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}

	cases := conformance.Cases()
	if *list {
		return writeConformanceCases(stdout, cases)
	}

	if err := verifyLocalConformance(cases); err != nil {
		return err
	}

	if *oracleCommand == "" {
		fmt.Fprintf(stdout, "local conformance cases passed: %d\n", len(cases))
		fmt.Fprintln(stdout, "no server oracle configured; set CONDITIONAL_ORACLE_COMMAND or pass --oracle-command to compare")
		return nil
	}

	return compareWithOracle(cases, *oracleCommand, *oracleTimeout, stdout, stderr)
}

func writeConformanceCases(w io.Writer, cases []conformance.Case) error {
	enc := json.NewEncoder(w)
	for _, c := range cases {
		if err := enc.Encode(conformance.OracleRequestFor(c)); err != nil {
			return fmt.Errorf("write case %q: %w", c.Name, err)
		}
	}
	return nil
}

func verifyLocalConformance(cases []conformance.Case) error {
	for _, c := range cases {
		expected := conformance.Expected(c)
		actual := conformance.EvaluateLocal(c)
		if err := conformance.Compare(c.Mode, expected, actual); err != nil {
			return fmt.Errorf("local conformance case %q failed: %w", c.Name, err)
		}
	}
	return nil
}

func compareWithOracle(cases []conformance.Case, command string, timeout time.Duration, stdout, stderr io.Writer) error {
	mismatches := 0
	for _, c := range cases {
		expected := conformance.Expected(c)
		actual, err := runOracle(command, timeout, conformance.OracleRequestFor(c))
		if err != nil {
			return fmt.Errorf("oracle failed for %q: %w", c.Name, err)
		}
		if err := conformance.Compare(c.Mode, expected, actual); err != nil {
			mismatches++
			fmt.Fprintf(stderr, "mismatch %q: %v\n", c.Name, err)
		}
	}
	if mismatches > 0 {
		return fmt.Errorf("server oracle mismatches: %d of %d", mismatches, len(cases))
	}

	fmt.Fprintf(stdout, "server oracle conformance passed: %d cases\n", len(cases))
	return nil
}

func runOracle(command string, timeout time.Duration, request conformance.OracleRequest) (conformance.Result, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return conformance.Result{}, fmt.Errorf("marshal request: %w", err)
	}

	ctx := context.Background()
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	cmd := exec.Command("sh", "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdin = bytes.NewReader(payload)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return conformance.Result{}, err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	var waitErr error
	select {
	case waitErr = <-done:
	case <-ctx.Done():
		killProcessGroup(cmd)
		<-done
		waitErr = ctx.Err()
	}

	if waitErr != nil {
		if ctx.Err() != nil {
			return conformance.Result{}, ctx.Err()
		}
		if stderr.Len() > 0 {
			return conformance.Result{}, fmt.Errorf("%w: %s", waitErr, strings.TrimSpace(stderr.String()))
		}
		return conformance.Result{}, waitErr
	}

	var result conformance.Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return conformance.Result{}, fmt.Errorf("decode oracle response: %w", err)
	}
	return result, nil
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
