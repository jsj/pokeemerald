package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/jsj/pokectx/internal/adapter"
)

func TestDoctorFailureIsStructuredOnce(t *testing.T) {
	var output bytes.Buffer
	err := writeDoctor(&output, adapter.Observation{}, errors.New("connection refused"))
	var reported reportedError
	if !errors.As(err, &reported) {
		t.Fatalf("error = %v", err)
	}
	if strings.Count(output.String(), `"schema_version"`) != 1 {
		t.Fatalf("output = %s", output.String())
	}
	if !strings.Contains(output.String(), `"status": "error"`) {
		t.Fatalf("output = %s", output.String())
	}
}

func TestShellQuotePreservesSpacesAndApostrophes(t *testing.T) {
	if got, want := shellQuote("James's Save/game.sav"), `'James'"'"'s Save/game.sav'`; got != want {
		t.Fatalf("shellQuote() = %q, want %q", got, want)
	}
}

func TestHelpUsesClearCommandNames(t *testing.T) {
	var output bytes.Buffer
	if err := run(t.Context(), nil, &output, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "team|bag") {
		t.Fatalf("output = %s", output.String())
	}
	if strings.Contains(output.String(), "party|inventory") {
		t.Fatalf("legacy names are in primary help: %s", output.String())
	}
}
