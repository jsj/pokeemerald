package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/jsj/pokectx/internal/adapter"
	"github.com/jsj/pokectx/internal/catalog"
	"github.com/jsj/pokectx/internal/game"
	"github.com/jsj/pokectx/internal/model"
	"github.com/jsj/pokectx/internal/savecloud"
)

const version = "0.1.0"

var primaryCommands = []string{
	"doctor",
	"status",
	"location",
	"team",
	"bag",
	"battle",
	"progress",
	"next",
	"snapshot",
	"watch",
	"save",
	"maps",
	"cheats",
}

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
		return writeRootUsage(stdout)
	}
	if args[0] == "maps" {
		return runMaps(args[1:], stdout)
	}
	if host := os.Getenv("POKECTX_SSH_HOST"); host != "" && os.Getenv("POKECTX_REMOTE") == "" {
		return runSSH(ctx, host, args, stdout, stderr)
	}
	if args[0] == "save" {
		return runSave(ctx, args[1:], stdout)
	}
	if args[0] == "cheats" {
		return runCheats(ctx, args[1:], stdout)
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
	case "team":
		result = struct {
			SchemaVersion int             `json:"schema_version"`
			Team          []model.Pokemon `json:"team"`
			Evidence      model.Evidence  `json:"evidence"`
		}{model.SchemaVersion, snapshot.Team, snapshot.Evidence}
	case "party": // Backward-compatible alias for team.
		result = struct {
			SchemaVersion int             `json:"schema_version"`
			Party         []model.Pokemon `json:"party"`
			Evidence      model.Evidence  `json:"evidence"`
		}{model.SchemaVersion, snapshot.Team, snapshot.Evidence}
	case "bag", "inventory":
		result = struct {
			SchemaVersion int            `json:"schema_version"`
			Money         uint32         `json:"money"`
			Items         []model.Item   `json:"items"`
			PokeBalls     []model.Item   `json:"poke_balls"`
			TMsAndHMs     []model.Item   `json:"tms_and_hms"`
			Berries       []model.Item   `json:"berries"`
			Badges        []string       `json:"badges"`
			KeyItems      []model.Item   `json:"key_items"`
			RepelSteps    uint16         `json:"repel_steps"`
			Evidence      model.Evidence `json:"evidence"`
		}{model.SchemaVersion, snapshot.Money, snapshot.Items, snapshot.PokeBalls, snapshot.TMsAndHMs, snapshot.Berries, snapshot.Badges, snapshot.KeyItems, snapshot.RepelSteps, snapshot.Evidence}
	case "battle":
		result = struct {
			SchemaVersion int            `json:"schema_version"`
			Battle        *model.Battle  `json:"battle"`
			Evidence      model.Evidence `json:"evidence"`
		}{model.SchemaVersion, snapshot.Battle, snapshot.Evidence}
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

func runMaps(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		_, _ = fmt.Fprintln(stdout, "usage: pokectx maps <list|find QUERY>")
		return nil
	}
	var query string
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return errors.New("usage: pokectx maps list")
		}
	case "find":
		if len(args) < 2 {
			return errors.New("usage: pokectx maps find QUERY")
		}
		query = strings.Join(args[1:], " ")
	default:
		return fmt.Errorf("unknown maps command %q", args[0])
	}
	maps := catalog.Default.MapEntries(query)
	result := struct {
		SchemaVersion int                `json:"schema_version"`
		Query         string             `json:"query,omitempty"`
		Count         int                `json:"count"`
		Maps          []catalog.MapEntry `json:"maps"`
	}{model.SchemaVersion, query, len(maps), maps}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

const pokectxCheatCRC32 = "8e82fa39"

func runCheats(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		_, _ = fmt.Fprintln(stdout, "usage: pokectx cheats <set-money AMOUNT|level POKEMON LEVEL|give-item ITEM QUANTITY|give SPECIES LEVEL [--shiny]|learn POKEMON MOVE|forget POKEMON MOVE|heal|teleport MAP X Y> [--confirm]")
		return nil
	}
	values, confirmed, shiny, err := parseCheatOptions(args)
	if err != nil {
		return err
	}
	address := os.Getenv("POKECTX_ADDRESS")
	if address == "" {
		address = "127.0.0.1:8765"
	}
	reader := adapter.SocketReader{Address: address, Timeout: 3 * time.Second}
	observation, err := reader.Observe(ctx)
	if err != nil {
		return err
	}
	snapshot, err := (game.Decoder{Catalog: catalog.Default}).Decode(observation)
	if err != nil {
		return err
	}
	if snapshot.Battle != nil {
		return errors.New("cheats are unavailable during battle")
	}
	request, before, after, err := prepareCheat(values, snapshot)
	if err != nil {
		return err
	}
	if shiny {
		if len(values) == 0 || values[0] != "give" {
			return errors.New("the --shiny option requires the give command")
		}
		request.Arg1 |= 1 << 31
		if pokemon, ok := after.(map[string]any); ok {
			pokemon["shiny"] = true
		}
	}
	result := map[string]any{
		"schema_version": model.SchemaVersion,
		"command":        strings.Join(values, " "),
		"before":         before,
		"after":          after,
		"applied":        false,
	}
	if confirmed {
		if strings.ToLower(observation.ROMCRC32) != pokectxCheatCRC32 {
			return errors.New("the running ROM does not include the Pokéctx cheat mailbox")
		}
		request.Nonce = uint32(time.Now().UnixNano())
		if _, err := reader.ExecuteCheat(ctx, request); err != nil {
			return err
		}
		result["applied"] = true
	} else {
		result["confirmation"] = "preview only. Repeat the command with --confirm to apply"
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func parseCheatOptions(args []string) ([]string, bool, bool, error) {
	confirmed := false
	shiny := false
	values := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "--confirm":
			confirmed = true
		case "--shiny":
			shiny = true
		default:
			if strings.HasPrefix(arg, "--") {
				return nil, false, false, fmt.Errorf("unknown option %q", arg)
			}
			values = append(values, arg)
		}
	}
	return values, confirmed, shiny, nil
}

func prepareCheat(args []string, snapshot model.Snapshot) (adapter.CheatRequest, any, any, error) {
	var request adapter.CheatRequest
	if len(args) == 0 {
		return request, nil, nil, errors.New("missing cheat command")
	}
	switch args[0] {
	case "set-money":
		if len(args) != 2 {
			return request, nil, nil, errors.New("usage: pokectx cheats set-money AMOUNT [--confirm]")
		}
		amount, err := parseUint(args[1], 999999)
		if err != nil {
			return request, nil, nil, fmt.Errorf("money: %w", err)
		}
		request.Command, request.Arg1 = 1, uint32(amount)
		return request, snapshot.Money, amount, nil
	case "level":
		if len(args) != 3 {
			return request, nil, nil, errors.New("usage: pokectx cheats level POKEMON LEVEL [--confirm]")
		}
		level, err := parseUint(args[2], 100)
		if err != nil || level == 0 {
			return request, nil, nil, errors.New("level must be between 1 and 100")
		}
		wanted := normalizedName(args[1])
		for index, pokemon := range snapshot.Team {
			if normalizedName(pokemon.Nickname) == wanted || normalizedName(pokemon.Species) == wanted {
				request.Command, request.Arg1, request.Arg2 = 2, uint32(index), uint16(level)
				return request, map[string]any{"pokemon": pokemon.Nickname, "level": pokemon.Level}, map[string]any{"pokemon": pokemon.Nickname, "level": level}, nil
			}
		}
		return request, nil, nil, fmt.Errorf("Pokémon %q is not in the team", args[1])
	case "give-item":
		if len(args) != 3 {
			return request, nil, nil, errors.New("usage: pokectx cheats give-item ITEM QUANTITY [--confirm]")
		}
		itemID, ok := catalog.Default.FindItem(args[1])
		if !ok {
			return request, nil, nil, fmt.Errorf("unknown item %q", args[1])
		}
		quantity, err := parseUint(args[2], 999)
		if err != nil || quantity == 0 {
			return request, nil, nil, errors.New("quantity must be between 1 and 999")
		}
		current := itemQuantity(snapshot, itemID)
		request.Command, request.Arg1, request.Arg2 = 3, uint32(itemID), uint16(quantity)
		return request, map[string]any{"item": catalog.Default.ItemName(itemID), "quantity": current}, map[string]any{"item": catalog.Default.ItemName(itemID), "quantity": current + int(quantity)}, nil
	case "give":
		if len(args) != 3 {
			return request, nil, nil, errors.New("usage: pokectx cheats give SPECIES LEVEL [--confirm]")
		}
		speciesID, ok := catalog.Default.FindSpecies(args[1])
		if !ok || speciesID == 0 || speciesID >= 412 {
			return request, nil, nil, fmt.Errorf("unknown Pokémon species %q", args[1])
		}
		level, err := parseUint(args[2], 100)
		if err != nil || level == 0 {
			return request, nil, nil, errors.New("level must be between 1 and 100")
		}
		pokemon := map[string]any{"species": catalog.Default.SpeciesName(speciesID), "level": level}
		request.Command, request.Arg1, request.Arg2 = 4, uint32(speciesID), uint16(level)
		return request, nil, pokemon, nil
	case "heal":
		if len(args) != 1 {
			return request, nil, nil, errors.New("usage: pokectx cheats heal [--confirm]")
		}
		afterTeam := make([]map[string]any, 0, len(snapshot.Team))
		for _, pokemon := range snapshot.Team {
			afterTeam = append(afterTeam, map[string]any{
				"pokemon": pokemon.Nickname,
				"hp":      pokemon.MaxHP,
				"status":  "healthy",
				"pp":      "full",
			})
		}
		request.Command = 7
		return request, snapshot.Team, afterTeam, nil
	case "teleport":
		if len(args) != 4 {
			return request, nil, nil, errors.New("usage: pokectx cheats teleport MAP X Y [--confirm]")
		}
		mapEntry, ok := catalog.Default.FindMap(args[1])
		if !ok {
			return request, nil, nil, fmt.Errorf("unknown map %q. Use pokectx maps find QUERY", args[1])
		}
		x, err := parseUint(args[2], 127)
		if err != nil {
			return request, nil, nil, fmt.Errorf("x coordinate: %w", err)
		}
		y, err := parseUint(args[3], 127)
		if err != nil {
			return request, nil, nil, fmt.Errorf("y coordinate: %w", err)
		}
		request.Command = 8
		request.Arg1 = uint32(mapEntry.MapGroup<<8 | mapEntry.MapNum)
		request.Arg2 = uint16(x | y<<8)
		after := map[string]any{
			"map_id":    mapEntry.ID,
			"name":      mapEntry.Name,
			"map_group": mapEntry.MapGroup,
			"map_num":   mapEntry.MapNum,
			"x":         x,
			"y":         y,
			"warning":   "Entering some maps can start story scripts out of order.",
		}
		return request, snapshot.Location, after, nil
	case "learn", "forget":
		if len(args) != 3 {
			return request, nil, nil, fmt.Errorf("usage: pokectx cheats %s POKEMON MOVE [--confirm]", args[0])
		}
		teamIndex, pokemon, ok := findTeamPokemon(snapshot, args[1])
		if !ok {
			return request, nil, nil, fmt.Errorf("Pokémon %q is not in the team", args[1])
		}
		moveID, ok := catalog.Default.FindMove(args[2])
		if !ok || moveID == 0 || moveID >= 355 {
			return request, nil, nil, fmt.Errorf("unknown move %q", args[2])
		}
		moveName := catalog.Default.MoveName(moveID)
		moveIndex := indexOfMove(pokemon, moveID)
		before := map[string]any{"pokemon": pokemon.Nickname, "moves": pokemon.Moves}
		afterMoves := append([]string(nil), pokemon.Moves...)
		if args[0] == "learn" {
			if moveIndex >= 0 {
				return request, nil, nil, fmt.Errorf("%s already knows %s", pokemon.Nickname, moveName)
			}
			if len(pokemon.MoveIDs) >= 4 {
				return request, nil, nil, fmt.Errorf("%s already knows four moves. Forget one move first", pokemon.Nickname)
			}
			request.Command = 5
			afterMoves = append(afterMoves, moveName)
		} else {
			if moveIndex < 0 {
				return request, nil, nil, fmt.Errorf("%s does not know %s", pokemon.Nickname, moveName)
			}
			if len(pokemon.MoveIDs) == 1 {
				return request, nil, nil, fmt.Errorf("%s cannot forget its last move", pokemon.Nickname)
			}
			request.Command = 6
			afterMoves = append(afterMoves[:moveIndex], afterMoves[moveIndex+1:]...)
		}
		request.Arg1, request.Arg2 = uint32(teamIndex), moveID
		return request, before, map[string]any{"pokemon": pokemon.Nickname, "moves": afterMoves}, nil
	default:
		return request, nil, nil, fmt.Errorf("unknown cheat command %q", args[0])
	}
}

func parseUint(value string, maximum uint64) (uint64, error) {
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil || parsed > maximum {
		return 0, fmt.Errorf("must be between 0 and %d", maximum)
	}
	return parsed, nil
}

func normalizedName(value string) string {
	return strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToUpper(value))
}

func findTeamPokemon(snapshot model.Snapshot, value string) (int, model.Pokemon, bool) {
	wanted := normalizedName(value)
	for index, pokemon := range snapshot.Team {
		if normalizedName(pokemon.Nickname) == wanted || normalizedName(pokemon.Species) == wanted {
			return index, pokemon, true
		}
	}
	return 0, model.Pokemon{}, false
}

func indexOfMove(pokemon model.Pokemon, moveID uint16) int {
	for index, knownMoveID := range pokemon.MoveIDs {
		if knownMoveID == moveID {
			return index
		}
	}
	return -1
}

func itemQuantity(snapshot model.Snapshot, id uint16) int {
	for _, pocket := range [][]model.Item{snapshot.Items, snapshot.KeyItems, snapshot.PokeBalls, snapshot.TMsAndHMs, snapshot.Berries} {
		for _, item := range pocket {
			if item.ID == id {
				return int(item.Quantity)
			}
		}
	}
	return 0
}

func writeRootUsage(output io.Writer) error {
	_, err := fmt.Fprintf(
		output,
		"usage: pokectx <%s> [options]\n",
		strings.Join(primaryCommands, "|"),
	)
	return err
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
		key := strings.Join([]string{snapshot.Location.MapID, fmt.Sprint(snapshot.Save.Counter), fmt.Sprint(len(snapshot.Badges)), fmt.Sprint(len(snapshot.Team))}, ":")
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
