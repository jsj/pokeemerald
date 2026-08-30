package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jsj/pokectx/internal/adapter"
	"github.com/jsj/pokectx/internal/catalog"
	"github.com/jsj/pokectx/internal/game"
	"github.com/jsj/pokectx/internal/model"
	"github.com/jsj/pokectx/internal/savecloud"
)

const version = "0.1.0"

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		var reported reportedError
		if errors.As(err, &reported) {
			os.Exit(reported.code)
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"schema_version": model.SchemaVersion, "status": "error", "error": err.Error()})
		os.Exit(2)
	}
}

type reportedError struct{ code int }

func (e reportedError) Error() string { return "command reported a structured failure" }

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		_, _ = fmt.Fprintln(stdout, "usage: pokectx <doctor|status|location|party|inventory|progress|next|snapshot|watch|save> [options]")
		return nil
	}
	if host := os.Getenv("POKECTX_SSH_HOST"); host != "" && os.Getenv("POKECTX_REMOTE") == "" {
		return runSSH(ctx, host, args, stdout, stderr)
	}
	if args[0] == "save" {
		return runSave(ctx, args[1:], stdout)
	}
	reader, err := selectReader(args)
	if err != nil {
		return err
	}
	if args[0] == "watch" {
		return watch(ctx, reader, stdout)
	}
	observation, err := reader.Observe(ctx)
	if args[0] == "doctor" {
		return writeDoctor(stdout, observation, err)
	}
	if err != nil {
		return err
	}
	snapshot, err := (game.Decoder{Catalog: catalog.Default}).Decode(observation)
	if err != nil {
		return err
	}
	var result any
	switch args[0] {
	case "snapshot", "status":
		result = snapshot
	case "location":
		result = struct {
			SchemaVersion int            `json:"schema_version"`
			Location      model.Location `json:"location"`
			Evidence      model.Evidence `json:"evidence"`
		}{model.SchemaVersion, snapshot.Location, snapshot.Evidence}
	case "party":
		result = struct {
			SchemaVersion int             `json:"schema_version"`
			Party         []model.Pokemon `json:"party"`
			Evidence      model.Evidence  `json:"evidence"`
		}{model.SchemaVersion, snapshot.Party, snapshot.Evidence}
	case "inventory":
		result = struct {
			SchemaVersion int            `json:"schema_version"`
			Badges        []string       `json:"badges"`
			KeyItems      []model.Item   `json:"key_items"`
			Evidence      model.Evidence `json:"evidence"`
		}{model.SchemaVersion, snapshot.Badges, snapshot.KeyItems, snapshot.Evidence}
	case "progress":
		result = struct {
			SchemaVersion int            `json:"schema_version"`
			Milestones    []string       `json:"milestones"`
			Evidence      model.Evidence `json:"evidence"`
		}{model.SchemaVersion, snapshot.Milestones, snapshot.Evidence}
	case "next":
		result = struct {
			SchemaVersion int             `json:"schema_version"`
			Next          model.Objective `json:"next"`
			Evidence      model.Evidence  `json:"evidence"`
		}{model.SchemaVersion, snapshot.Next, snapshot.Evidence}
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func runSave(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		_, _ = fmt.Fprintln(stdout, "usage: pokectx save <export --to FILE|import --from FILE|backup|sync> [--save FILE]")
		return nil
	}
	localPath, err := savecloud.DefaultSavePath()
	if err != nil {
		return err
	}
	var source, destination string
	for index := 1; index < len(args); index++ {
		if index+1 >= len(args) {
			return fmt.Errorf("%s needs a value", args[index])
		}
		value := args[index+1]
		switch args[index] {
		case "--save":
			localPath = value
		case "--from":
			source = value
		case "--to":
			destination = value
		default:
			return fmt.Errorf("unknown option %q", args[index])
		}
		index++
	}
	address := os.Getenv("POKECTX_ADDRESS")
	var result savecloud.Result
	switch args[0] {
	case "export":
		if destination == "" {
			return errors.New("save export needs --to FILE")
		}
		result, err = savecloud.Export(ctx, localPath, destination)
	case "import":
		if source == "" {
			return errors.New("save import needs --from FILE")
		}
		result, err = savecloud.Import(ctx, source, localPath, address)
	case "backup":
		var cloud *savecloud.R2
		cloud, err = savecloud.NewR2(ctx)
		if err == nil {
			var local savecloud.File
			local, err = savecloud.ReadValid(ctx, localPath)
			if err == nil {
				result, err = cloud.Backup(ctx, local)
			}
		}
	case "sync":
		var cloud *savecloud.R2
		cloud, err = savecloud.NewR2(ctx)
		if err == nil {
			result, err = cloud.Sync(ctx, localPath, address)
		}
	default:
		return fmt.Errorf("unknown save command %q", args[0])
	}
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func selectReader(args []string) (adapter.Reader, error) {
	for index := 1; index < len(args); index++ {
		if args[index] != "--save" {
			return nil, fmt.Errorf("unknown option %q", args[index])
		}
		if index+1 >= len(args) {
			return nil, fmt.Errorf("--save needs a file path")
		}
		return adapter.SaveFileReader{Path: args[index+1]}, nil
	}
	if saveFile := os.Getenv("POKECTX_SAVE_FILE"); saveFile != "" {
		return adapter.SaveFileReader{Path: saveFile}, nil
	}
	if fixture := os.Getenv("POKECTX_FIXTURE"); fixture != "" {
		return adapter.FixtureReader{Path: fixture}, nil
	}
	address := os.Getenv("POKECTX_ADDRESS")
	if address == "" {
		address = "127.0.0.1:8765"
	}
	return adapter.SocketReader{Address: address, Timeout: 3 * time.Second}, nil
}

func runSSH(ctx context.Context, host string, args []string, stdout, stderr io.Writer) error {
	remoteCommand := os.Getenv("POKECTX_REMOTE_COMMAND")
	if remoteCommand == "" {
		remoteCommand = "pokectx"
	}
	remoteParts := append([]string{"env", "POKECTX_REMOTE=1", remoteCommand}, args...)
	for index := range remoteParts {
		remoteParts[index] = shellQuote(remoteParts[index])
	}
	command := exec.CommandContext(ctx, "ssh", host, strings.Join(remoteParts, " "))
	command.Stdout, command.Stderr = stdout, stderr
	if err := command.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return reportedError{code: exitErr.ExitCode()}
		}
		return fmt.Errorf("run Pokéctx through SSH: %w", err)
	}
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func writeDoctor(w io.Writer, observation adapter.Observation, observationErr error) error {
	doctor := model.Doctor{SchemaVersion: model.SchemaVersion, Status: "ok", Version: version, Target: "local"}
	if os.Getenv("POKECTX_REMOTE") != "" {
		doctor.Target = "ssh"
	}
	doctor.Checks = append(doctor.Checks, model.Check{Name: "cli", Status: "ok", Detail: "Pokéctx command is available."})
	if observationErr != nil {
		doctor.Status = "error"
		doctor.Checks = append(doctor.Checks, model.Check{Name: "adapter", Status: "error", Detail: observationErr.Error(), Fix: "Load the Pokéctx Lua script in mGBA, then keep the game open."})
	} else {
		doctor.GameCode, doctor.ROMCRC32, doctor.ObservedAt = observation.GameCode, observation.ROMCRC32, &observation.ObservedAt
		adapterDetail := "The mGBA adapter returned live memory."
		if observation.Source == "save_file" {
			adapterDetail = "The Emerald save-file adapter returned a valid save."
		}
		doctor.Checks = append(doctor.Checks, model.Check{Name: "adapter", Status: "ok", Detail: adapterDetail})
		gameStatus := "ok"
		detail := "Pokémon Emerald (USA) is connected."
		if observation.GameCode != "BPEE" && observation.GameCode != "AGB-BPEE" {
			gameStatus, doctor.Status, detail = "error", "error", "The connected game is not supported."
		} else if observation.ROMCRC32 != "" && strings.ToLower(observation.ROMCRC32) != "1f1c08fb" {
			gameStatus, doctor.Status, detail = "error", "error", "The connected Emerald ROM revision is not supported."
		}
		doctor.Checks = append(doctor.Checks, model.Check{Name: "game", Status: gameStatus, Detail: detail})
		if snapshot, decodeErr := (game.Decoder{Catalog: catalog.Default}).Decode(observation); decodeErr == nil {
			lastSeen := observation.ObservedAt
			snapshot.Save.LastSeenAt = &lastSeen
			doctor.Save = &snapshot.Save
		} else {
			if doctor.Status != "error" {
				doctor.Status = "warning"
			}
			doctor.Checks = append(doctor.Checks, model.Check{Name: "game_state", Status: "warning", Detail: decodeErr.Error(), Fix: "Continue or start the game, then run pokectx doctor again."})
			lastSeen := observation.ObservedAt
			source := "live_memory"
			if observation.Source == "save_file" {
				source = "save_file"
			}
			doctor.Save = &model.SaveEvidence{Counter: observation.SaveCounter, LastSeenAt: &lastSeen, Source: source, FileModifiedAt: observation.FileModifiedAt}
		}
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(doctor); err != nil {
		return err
	}
	if observationErr != nil {
		return reportedError{code: 2}
	}
	if doctor.Status == "error" {
		return reportedError{code: 3}
	}
	return nil
}

func watch(ctx context.Context, reader adapter.Reader, w io.Writer) error {
	encoder := json.NewEncoder(w)
	var previous string
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		observation, err := reader.Observe(ctx)
		if err != nil {
			return err
		}
		snapshot, err := (game.Decoder{Catalog: catalog.Default}).Decode(observation)
		if err != nil {
			return err
		}
		key := strings.Join([]string{snapshot.Location.MapID, fmt.Sprint(snapshot.Save.Counter), fmt.Sprint(len(snapshot.Badges)), fmt.Sprint(len(snapshot.Party))}, ":")
		if key != previous {
			if err := encoder.Encode(snapshot); err != nil {
				return err
			}
			previous = key
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
