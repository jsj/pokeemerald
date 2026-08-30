package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/jsj/pokectx/internal/adapter"
	"github.com/jsj/pokectx/internal/model"
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

func TestPrepareSetLevelCheatUsesTeamSlot(t *testing.T) {
	snapshot := model.Snapshot{Team: []model.Pokemon{{Nickname: "MARSHTOMP", Species: "Marshtomp", Level: 22}}}
	request, before, after, err := prepareCheat([]string{"set-level", "marshtomp", "25"}, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if request.Command != 2 || request.Arg1 != 0 || request.Arg2 != 25 {
		t.Fatalf("request = %#v", request)
	}
	if before == nil || after == nil {
		t.Fatal("preview values are missing")
	}
}

func TestPrepareGiveItemCheatAcceptsHumanName(t *testing.T) {
	request, _, _, err := prepareCheat([]string{"give-item", "Super Potion", "3"}, model.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	if request.Command != 3 || request.Arg1 != 22 || request.Arg2 != 3 {
		t.Fatalf("request = %#v", request)
	}
}

func TestPrepareGivePokemonCheatAcceptsSpeciesName(t *testing.T) {
	request, before, after, err := prepareCheat([]string{"give", "Rayquaza", "70"}, model.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	if request.Command != 4 || request.Arg1 != 406 || request.Arg2 != 70 {
		t.Fatalf("request = %#v", request)
	}
	if before != nil || after == nil {
		t.Fatalf("before = %#v, after = %#v", before, after)
	}
}

func TestPrepareGivePokemonCheatKeepsOldAlias(t *testing.T) {
	request, _, _, err := prepareCheat([]string{"give-pokemon", "Rayquaza", "70"}, model.Snapshot{})
	if err != nil || request.Command != 4 {
		t.Fatalf("request = %#v, error = %v", request, err)
	}
}

func TestPrepareCheatRejectsInvalidValues(t *testing.T) {
	for _, args := range [][]string{{"set-money", "1000000"}, {"set-level", "MARSHTOMP", "0"}, {"give-item", "REPEL", "0"}, {"give", "RAYQUAZA", "101"}, {"give", "MISSINGNO", "70"}, {"give", "EGG", "5"}} {
		if _, _, _, err := prepareCheat(args, model.Snapshot{}); err == nil {
			t.Fatalf("prepareCheat(%v) succeeded", args)
		}
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
