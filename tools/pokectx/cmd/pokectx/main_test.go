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

func TestMapsFindUsesDecompCatalogWithoutGameConnection(t *testing.T) {
	var output bytes.Buffer
	if err := run(t.Context(), []string{"maps", "find", "sky pillar"}, &output, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"id": "MAP_SKY_PILLAR_3_F"`) {
		t.Fatalf("output = %s", output.String())
	}
	if strings.Contains(output.String(), `MAP_GRANITE_CAVE`) {
		t.Fatalf("unrelated map is in output: %s", output.String())
	}
}

func TestMapsListIncludesMapCoordinates(t *testing.T) {
	var output bytes.Buffer
	if err := run(t.Context(), []string{"maps", "list"}, &output, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for _, wanted := range []string{`"id": "MAP_SLATEPORT_CITY"`, `"map_group": 0`, `"map_num": 1`} {
		if !strings.Contains(output.String(), wanted) {
			t.Fatalf("output does not contain %s", wanted)
		}
	}
}

func TestPrepareLevelCheatUsesTeamSlot(t *testing.T) {
	snapshot := model.Snapshot{Team: []model.Pokemon{{Nickname: "MARSHTOMP", Species: "Marshtomp", Level: 22}}}
	request, before, after, err := prepareCheat([]string{"level", "marshtomp", "25"}, snapshot)
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

func TestPrepareLearnCheatUsesTeamSlotAndMove(t *testing.T) {
	snapshot := model.Snapshot{Team: []model.Pokemon{{Nickname: "MARSHTOMP", MoveIDs: []uint16{33, 55, 189}, Moves: []string{"Tackle", "Water Gun", "Mud Slap"}}}}
	request, before, after, err := prepareCheat([]string{"learn", "marshtomp", "Ice Beam"}, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if request.Command != 5 || request.Arg1 != 0 || request.Arg2 != 58 {
		t.Fatalf("request = %#v", request)
	}
	if before == nil || after == nil {
		t.Fatal("preview values are missing")
	}
}

func TestPrepareForgetCheatUsesKnownMove(t *testing.T) {
	snapshot := model.Snapshot{Team: []model.Pokemon{{Nickname: "MARSHTOMP", MoveIDs: []uint16{33, 55}, Moves: []string{"Tackle", "Water Gun"}}}}
	request, before, after, err := prepareCheat([]string{"forget", "marshtomp", "Tackle"}, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if request.Command != 6 || request.Arg1 != 0 || request.Arg2 != 33 {
		t.Fatalf("request = %#v", request)
	}
	if before == nil || after == nil {
		t.Fatal("preview values are missing")
	}
}

func TestPrepareMoveCheatsRejectUnsafeChanges(t *testing.T) {
	full := model.Snapshot{Team: []model.Pokemon{{Nickname: "MARSHTOMP", MoveIDs: []uint16{33, 55, 189, 341}, Moves: []string{"Tackle", "Water Gun", "Mud Slap", "Mud Shot"}}}}
	if _, _, _, err := prepareCheat([]string{"learn", "MARSHTOMP", "ICE_BEAM"}, full); err == nil {
		t.Fatal("learn succeeded with four known moves")
	}
	one := model.Snapshot{Team: []model.Pokemon{{Nickname: "MARSHTOMP", MoveIDs: []uint16{33}, Moves: []string{"Tackle"}}}}
	if _, _, _, err := prepareCheat([]string{"forget", "MARSHTOMP", "TACKLE"}, one); err == nil {
		t.Fatal("forget removed the last move")
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

func TestParseCheatOptionsFindsShinyAndConfirmation(t *testing.T) {
	values, confirmed, shiny, err := parseCheatOptions([]string{"give", "RAYQUAZA", "70", "--shiny", "--confirm"})
	if err != nil {
		t.Fatal(err)
	}
	if !confirmed || !shiny || strings.Join(values, " ") != "give RAYQUAZA 70" {
		t.Fatalf("values = %v, confirmed = %v, shiny = %v", values, confirmed, shiny)
	}
}

func TestPrepareHealCheat(t *testing.T) {
	snapshot := model.Snapshot{Team: []model.Pokemon{{Nickname: "MARSHTOMP", HP: 10, MaxHP: 71, Status: "poison"}}}
	request, before, after, err := prepareCheat([]string{"heal"}, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if request.Command != 7 || before == nil || after == nil {
		t.Fatalf("request = %#v, before = %#v, after = %#v", request, before, after)
	}
}

func TestPrepareTeleportCheatPacksMapAndCoordinates(t *testing.T) {
	request, before, after, err := prepareCheat([]string{"teleport", "SLATEPORT_CITY", "19", "20"}, model.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	if request.Command != 8 || request.Arg1 != 1 || request.Arg2 != 0x1413 {
		t.Fatalf("request = %#v", request)
	}
	if before == nil || after == nil {
		t.Fatal("preview values are missing")
	}
}

func TestPreparePokemonAttributeCheats(t *testing.T) {
	snapshot := model.Snapshot{Team: []model.Pokemon{{Nickname: "MARSHTOMP", Species: "Marshtomp", Friendship: 70}}}
	tests := []struct {
		args    []string
		command uint8
		arg2    uint16
	}{
		{[]string{"hold", "MARSHTOMP", "LEFTOVERS"}, 9, 200},
		{[]string{"friendship", "MARSHTOMP", "255"}, 10, 255},
		{[]string{"clone", "MARSHTOMP"}, 11, 0},
		{[]string{"evolve", "MARSHTOMP", "SWAMPERT"}, 12, 285},
		{[]string{"badge", "STONE"}, 14, 0},
	}
	for _, test := range tests {
		request, _, _, err := prepareCheat(test.args, snapshot)
		if err != nil {
			t.Fatalf("prepareCheat(%v): %v", test.args, err)
		}
		if request.Command != test.command || request.Arg1 != 0 || request.Arg2 != test.arg2 {
			t.Fatalf("prepareCheat(%v) = %#v", test.args, request)
		}
	}
}

func TestPrepareRenameCheatEncodesGameText(t *testing.T) {
	snapshot := model.Snapshot{Team: []model.Pokemon{{Nickname: "MARSHTOMP"}}}
	request, before, after, err := prepareCheat([]string{"rename", "MARSHTOMP", "Mud-2"}, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0xC7, 0xE9, 0xD8, 0xAE, 0xA3, 0xFF}
	if request.Command != 13 || !bytes.Equal(request.Data, want) {
		t.Fatalf("request = %#v", request)
	}
	if before != "MARSHTOMP" || after != "Mud-2" {
		t.Fatalf("before = %#v, after = %#v", before, after)
	}
}

func TestPrepareAllBadgesCheat(t *testing.T) {
	request, before, after, err := prepareCheat([]string{"badge", "ALL"}, model.Snapshot{Badges: []string{"Stone Badge"}})
	if err != nil {
		t.Fatal(err)
	}
	if request.Command != 14 || request.Arg1 != 0xFF || before == nil || after == nil {
		t.Fatalf("request = %#v, before = %#v, after = %#v", request, before, after)
	}
}

func TestPrepareCheatRejectsInvalidValues(t *testing.T) {
	for _, args := range [][]string{{"set-money", "1000000"}, {"level", "MARSHTOMP", "0"}, {"set-level", "MARSHTOMP", "25"}, {"give-item", "REPEL", "0"}, {"give", "RAYQUAZA", "101"}, {"give", "MISSINGNO", "70"}, {"give", "EGG", "5"}, {"give-pokemon", "RAYQUAZA", "70"}, {"grant-pokemon", "RAYQUAZA", "70"}} {
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
