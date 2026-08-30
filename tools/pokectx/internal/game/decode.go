package game

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/jsj/pokectx/internal/adapter"
	"github.com/jsj/pokectx/internal/catalog"
	"github.com/jsj/pokectx/internal/model"
)

const (
	saveBlock1Size  = 0x3D88
	saveBlock2Size  = 0xF2C
	teamPokemonSize = 100
	supportedCRC32  = "1f1c08fb"
	pokectxCRC32    = "8fc08ba1"
	statusSleep     = 0x7
	statusPoison    = 0x8
	statusBurn      = 0x10
	statusFreeze    = 0x20
	statusParalyze  = 0x40
	statusToxic     = 0x80
)

type Decoder struct{ Catalog catalog.Catalog }

func (d Decoder) Decode(observation adapter.Observation) (model.Snapshot, error) {
	sb1, err := decodeBlock(observation.SaveBlock1Hex, saveBlock1Size, "SaveBlock1")
	if err != nil {
		return model.Snapshot{}, err
	}
	sb2, err := decodeBlock(observation.SaveBlock2Hex, saveBlock2Size, "SaveBlock2")
	if err != nil {
		return model.Snapshot{}, err
	}
	teamData, err := decodeBlock(observation.PartyHex, int(observation.PartyCount)*teamPokemonSize, "team")
	if err != nil {
		return model.Snapshot{}, err
	}
	if observation.GameCode != "BPEE" && observation.GameCode != "AGB-BPEE" {
		return model.Snapshot{}, fmt.Errorf("unsupported game code %q", observation.GameCode)
	}
	crc := strings.ToLower(observation.ROMCRC32)
	if crc != "" && crc != supportedCRC32 && crc != pokectxCRC32 {
		return model.Snapshot{}, fmt.Errorf("unsupported Pokémon Emerald ROM CRC32 %q", observation.ROMCRC32)
	}

	group, number := int(int8(sb1[4])), int(int8(sb1[5]))
	mapID, mapName := d.Catalog.Map(group, number)
	playerName := decodeText(sb2[:8])
	if strings.TrimSpace(playerName) == "" && observation.PartyCount == 0 && binary.LittleEndian.Uint16(sb2[0x0E:0x10]) == 0 {
		return model.Snapshot{}, fmt.Errorf("game state is not ready; continue or start a game, then try again")
	}
	source := observation.Source
	if source == "" {
		source = "mgba_live_memory"
	}
	saveSource := "live_memory"
	if source == "save_file" {
		saveSource = "save_file"
	}
	encryptionKey := binary.LittleEndian.Uint32(sb2[0xAC:0xB0])
	story := decodeStory(sb1)
	snapshot := model.Snapshot{
		SchemaVersion: model.SchemaVersion,
		GameCode:      observation.GameCode,
		ROMCRC32:      observation.ROMCRC32,
		PlayerName:    playerName,
		Money:         binary.LittleEndian.Uint32(sb1[0x490:0x494]) ^ encryptionKey,
		Location:      model.Location{MapGroup: group, MapNum: number, MapID: mapID, Name: mapName, X: int(int16(binary.LittleEndian.Uint16(sb1[0:2]))), Y: int(int16(binary.LittleEndian.Uint16(sb1[2:4])))},
		Team:          d.decodeTeam(teamData, int(observation.PartyCount)),
		Items:         d.decodeItemPocket(sb1, 0x560, 30, encryptionKey),
		Badges:        decodeBadges(sb1),
		KeyItems:      d.decodeItemPocket(sb1, 0x5D8, 30, encryptionKey),
		PokeBalls:     d.decodeItemPocket(sb1, 0x650, 16, encryptionKey),
		TMsAndHMs:     d.decodeItemPocket(sb1, 0x690, 64, encryptionKey),
		Berries:       d.decodeItemPocket(sb1, 0x790, 46, encryptionKey),
		RepelSteps:    variable(sb1, 0x4021),
		Story:         story,
		Save:          model.SaveEvidence{Counter: observation.SaveCounter, PlayTime: fmt.Sprintf("%d:%02d:%02d", binary.LittleEndian.Uint16(sb2[0x0E:0x10]), sb2[0x10], sb2[0x11]), Source: saveSource, FileModifiedAt: observation.FileModifiedAt},
		Evidence:      model.Evidence{Source: source, ObservedAt: observation.ObservedAt, Freshness: freshness(observation.ObservedAt)},
	}
	snapshot.Milestones, snapshot.Next = progress(sb1, story)
	if observation.InBattle {
		battle, err := d.decodeBattle(observation)
		if err != nil {
			return model.Snapshot{}, err
		}
		snapshot.Battle = &battle
	}
	return snapshot, nil
}

func decodeBlock(value string, minimum int, label string) ([]byte, error) {
	data, err := hex.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", label, err)
	}
	if len(data) < minimum {
		return nil, fmt.Errorf("%s is %d bytes; need %d", label, len(data), minimum)
	}
	return data, nil
}

func freshness(observed time.Time) string {
	age := time.Since(observed)
	if age < 0 || age <= 5*time.Second {
		return "live"
	}
	if age <= 30*time.Second {
		return "recent"
	}
	return "stale"
}

func (d Decoder) decodeTeam(data []byte, count int) []model.Pokemon {
	if count > 6 {
		count = 6
	}
	team := make([]model.Pokemon, 0, count)
	for i := 0; i < count; i++ {
		mon := data[i*teamPokemonSize : (i+1)*teamPokemonSize]
		personality := binary.LittleEndian.Uint32(mon[0:4])
		key := personality ^ binary.LittleEndian.Uint32(mon[4:8])
		plain := make([]byte, 48)
		for offset := 0; offset < 48; offset += 4 {
			binary.LittleEndian.PutUint32(plain[offset:offset+4], binary.LittleEndian.Uint32(mon[32+offset:36+offset])^key)
		}
		order := substructOrders[personality%24]
		growth := plain[orderIndex(order, 0)*12:]
		attacks := plain[orderIndex(order, 1)*12:]
		speciesID := binary.LittleEndian.Uint16(growth[0:2])
		heldItemID := binary.LittleEndian.Uint16(growth[2:4])
		moveIDs := make([]uint16, 0, 4)
		moveNames := make([]string, 0, 4)
		movePP := make([]int, 0, 4)
		for move := 0; move < 4; move++ {
			id := binary.LittleEndian.Uint16(attacks[move*2 : move*2+2])
			if id == 0 {
				continue
			}
			moveIDs = append(moveIDs, id)
			moveNames = append(moveNames, d.Catalog.MoveName(id))
			movePP = append(movePP, int(attacks[8+move]))
		}
		heldItem := ""
		if heldItemID != 0 {
			heldItem = d.Catalog.ItemName(heldItemID)
		}
		team = append(team, model.Pokemon{
			Slot:       i + 1,
			SpeciesID:  speciesID,
			Species:    d.Catalog.SpeciesName(speciesID),
			Nickname:   decodeText(mon[8:18]),
			Level:      mon[84],
			HP:         binary.LittleEndian.Uint16(mon[86:88]),
			MaxHP:      binary.LittleEndian.Uint16(mon[88:90]),
			Experience: binary.LittleEndian.Uint32(growth[4:8]),
			HeldItemID: heldItemID,
			HeldItem:   heldItem,
			MoveIDs:    moveIDs,
			Moves:      moveNames,
			MovePP:     movePP,
			IsEgg:      mon[19]&(1<<2) != 0,
			Status:     decodeStatus(binary.LittleEndian.Uint32(mon[80:84])),
		})
	}
	return team
}

func variable(saveBlock []byte, id uint16) uint16 {
	const variablesOffset = 0x139C
	const variablesStart = 0x4000
	offset := variablesOffset + int(id-variablesStart)*2
	return binary.LittleEndian.Uint16(saveBlock[offset : offset+2])
}

func decodeStatus(status uint32) string {
	switch {
	case status&statusSleep != 0:
		return "sleep"
	case status&statusToxic != 0:
		return "toxic_poison"
	case status&statusPoison != 0:
		return "poison"
	case status&statusBurn != 0:
		return "burn"
	case status&statusFreeze != 0:
		return "freeze"
	case status&statusParalyze != 0:
		return "paralysis"
	default:
		return "healthy"
	}
}

func (d Decoder) decodeBattle(observation adapter.Observation) (model.Battle, error) {
	const battlePokemonSize = 0x58
	data, err := decodeBlock(observation.BattleMonsHex, int(observation.BattlersCount)*battlePokemonSize, "battle Pokémon")
	if err != nil {
		return model.Battle{}, err
	}
	kind := "wild"
	if observation.BattleTypeFlags&(1<<3) != 0 {
		kind = "trainer"
	}
	battle := model.Battle{Kind: kind, CanAttemptRun: kind == "wild", Pokemon: make([]model.BattlePokemon, 0, observation.BattlersCount)}
	for battler := 0; battler < int(observation.BattlersCount); battler++ {
		mon := data[battler*battlePokemonSize : (battler+1)*battlePokemonSize]
		moveIDs := make([]uint16, 0, 4)
		moves := make([]string, 0, 4)
		movePP := make([]int, 0, 4)
		for slot := 0; slot < 4; slot++ {
			moveID := binary.LittleEndian.Uint16(mon[0x0C+slot*2 : 0x0E+slot*2])
			if moveID == 0 {
				continue
			}
			moveIDs = append(moveIDs, moveID)
			moves = append(moves, d.Catalog.MoveName(moveID))
			movePP = append(movePP, int(mon[0x24+slot]))
		}
		side := "player"
		if battler%2 == 1 {
			side = "opponent"
		}
		battle.Pokemon = append(battle.Pokemon, model.BattlePokemon{
			Battler:    battler,
			Side:       side,
			SpeciesID:  binary.LittleEndian.Uint16(mon[0:2]),
			Species:    d.Catalog.SpeciesName(binary.LittleEndian.Uint16(mon[0:2])),
			Level:      mon[0x2A],
			HP:         binary.LittleEndian.Uint16(mon[0x28:0x2A]),
			MaxHP:      binary.LittleEndian.Uint16(mon[0x2C:0x2E]),
			Types:      battleTypeNames(mon[0x21], mon[0x22]),
			AbilityID:  mon[0x20],
			Ability:    d.Catalog.AbilityName(uint16(mon[0x20])),
			MoveIDs:    moveIDs,
			Moves:      moves,
			MovePP:     movePP,
			Status:     decodeStatus(binary.LittleEndian.Uint32(mon[0x4C:0x50])),
			StatStages: decodeStatStages(mon[0x18:0x20]),
		})
	}
	return battle, nil
}

func battleTypeNames(first, second uint8) []string {
	names := []string{"Normal", "Fighting", "Flying", "Poison", "Ground", "Rock", "Bug", "Ghost", "Steel", "Mystery", "Fire", "Water", "Grass", "Electric", "Psychic", "Ice", "Dragon", "Dark"}
	result := []string{battleTypeName(names, first)}
	if second != first {
		result = append(result, battleTypeName(names, second))
	}
	return result
}

func battleTypeName(names []string, id uint8) string {
	if int(id) >= len(names) {
		return fmt.Sprintf("Unknown type %d", id)
	}
	return names[id]
}

func decodeStatStages(values []byte) map[string]int {
	names := []string{"hp", "attack", "defense", "speed", "special_attack", "special_defense", "accuracy", "evasion"}
	stages := make(map[string]int, len(names))
	for index, name := range names {
		stages[name] = int(values[index]) - 6
	}
	return stages
}

func (d Decoder) decodeItemPocket(sb1 []byte, offset, count int, encryptionKey uint32) []model.Item {
	items := make([]model.Item, 0)
	for i := 0; i < count; i++ {
		slot := sb1[offset+i*4 : offset+i*4+4]
		id := binary.LittleEndian.Uint16(slot[0:2])
		if id == 0 {
			continue
		}
		quantity := binary.LittleEndian.Uint16(slot[2:4]) ^ uint16(encryptionKey)
		items = append(items, model.Item{ID: id, Name: d.Catalog.ItemName(id), Quantity: quantity})
	}
	return items
}

func decodeBadges(sb1 []byte) []string {
	names := []string{"Stone Badge", "Knuckle Badge", "Dynamo Badge", "Heat Badge", "Balance Badge", "Feather Badge", "Mind Badge", "Rain Badge"}
	badges := make([]string, 0, 8)
	for i, name := range names {
		if flag(sb1, uint16(0x867+i)) {
			badges = append(badges, name)
		}
	}
	return badges
}

func flag(sb1 []byte, id uint16) bool {
	const flagsOffset = 0x1270
	index := flagsOffset + int(id/8)
	return index < len(sb1) && sb1[index]&(1<<(id%8)) != 0
}

func progress(sb1 []byte, story model.StoryState) ([]string, model.Objective) {
	if !story.RescuedBirch {
		return nil, objective("rescue_birch", "Rescue Professor Birch and choose a starter", "Route 101", "FLAG_RESCUED_BIRCH is not set")
	}
	if story.BirchLab < 3 {
		return nil, objective("talk_to_birch", "Talk to Professor Birch", "Professor Birch's Lab", "VAR_BIRCH_LAB_STATE is less than 3")
	}
	if !story.DefeatedRivalRoute103 {
		return nil, objective("battle_rival_103", "Find and battle May", "Route 103", "FLAG_DEFEATED_RIVAL_ROUTE103 is not set")
	}
	if !story.AdventureStarted {
		return nil, objective("get_pokedex", "Return to Professor Birch", "Professor Birch's Lab", "FLAG_ADVENTURE_STARTED is not set")
	}
	if story.PetalburgGym < 2 {
		return nil, objective("meet_norman", "Meet Norman and help Wally", "Petalburg City Gym", "VAR_PETALBURG_GYM_STATE is less than 2")
	}
	if !flag(sb1, 0x4F0) {
		return nil, objective("gym_4f0", "Challenge Roxanne at the Rustboro Gym", "Rustboro City Gym", "Stone Badge flag is not set")
	}
	if !story.RecoveredDevonGoods {
		return []string{"Defeated Roxanne"}, objective("recover_devon_goods", "Recover the Devon Goods and rescue Peeko", "Rusturf Tunnel", "FLAG_RECOVERED_DEVON_GOODS is not set")
	}
	if !story.ReturnedDevonGoods {
		return []string{"Defeated Roxanne"}, objective("return_devon_goods", "Return the Devon Goods to the researcher", "Route 116", "FLAG_RETURNED_DEVON_GOODS is not set")
	}
	if !story.ReceivedPokeNav {
		return []string{"Defeated Roxanne", "Recovered the Devon Goods"}, objective("meet_devon_president", "Meet the Devon president", "Devon Corporation", "FLAG_RECEIVED_POKENAV is not set")
	}
	if !flag(sb1, 0x4F1) {
		return []string{"Defeated Roxanne", "Recovered the Devon Goods"}, objective("gym_4f1", "Sail to Dewford and challenge Brawly", "Dewford Town Gym", "Knuckle Badge flag is not set")
	}
	if !story.DeliveredDevonGoods {
		return []string{"Defeated Roxanne", "Defeated Brawly"}, objective("deliver_devon_goods", "Deliver the Devon Goods to Captain Stern", "Slateport Oceanic Museum", "FLAG_DELIVERED_DEVON_GOODS is not set")
	}
	type gym struct {
		flag                         uint16
		milestone, next, destination string
	}
	gyms := []gym{
		{0x4F0, "Defeated Roxanne", "Challenge Roxanne at the Rustboro Gym", "Rustboro City Gym"},
		{0x4F1, "Defeated Brawly", "Challenge Brawly at the Dewford Gym", "Dewford Town Gym"},
		{0x4F2, "Defeated Wattson", "Challenge Wattson at the Mauville Gym", "Mauville City Gym"},
		{0x4F3, "Defeated Flannery", "Challenge Flannery at the Lavaridge Gym", "Lavaridge Town Gym"},
		{0x4F4, "Defeated Norman", "Challenge Norman at the Petalburg Gym", "Petalburg City Gym"},
		{0x4F5, "Defeated Winona", "Challenge Winona at the Fortree Gym", "Fortree City Gym"},
		{0x4F6, "Defeated Tate and Liza", "Challenge Tate and Liza at the Mossdeep Gym", "Mossdeep City Gym"},
		{0x4F7, "Defeated Juan", "Challenge Juan at the Sootopolis Gym", "Sootopolis City Gym"},
	}
	milestones := make([]string, 0, 12)
	for _, gym := range gyms {
		if flag(sb1, gym.flag) {
			milestones = append(milestones, gym.milestone)
			continue
		}
		return milestones, model.Objective{ID: fmt.Sprintf("gym_%x", gym.flag), Title: gym.next, Destination: gym.destination, Evidence: []string{fmt.Sprintf("story flag 0x%X is not set", gym.flag)}, Confidence: "medium"}
	}
	return milestones, model.Objective{ID: "pokemon_league", Title: "Continue to the Pokémon League", Destination: "Ever Grande City", Evidence: []string{"all eight Gym Leader flags are set"}, Confidence: "medium"}
}

func objective(id, title, destination, evidence string) model.Objective {
	return model.Objective{ID: id, Title: title, Destination: destination, Evidence: []string{evidence}, Confidence: "high"}
}

func decodeStory(sb1 []byte) model.StoryState {
	return model.StoryState{
		BirchLab:                 variable(sb1, 0x4084),
		PetalburgGym:             variable(sb1, 0x4085),
		RustboroCity:             variable(sb1, 0x405A),
		SlateportCity:            variable(sb1, 0x4058),
		MossdeepSpaceCenter:      variable(sb1, 0x409F),
		SootopolisWallace:        variable(sb1, 0x40D8),
		RescuedBirch:             flag(sb1, 0x52),
		AdventureStarted:         flag(sb1, 0x74),
		DefeatedRivalRoute103:    flag(sb1, 0x82),
		RecoveredDevonGoods:      flag(sb1, 0x8F),
		ReturnedDevonGoods:       flag(sb1, 0x90),
		ReceivedPokeNav:          flag(sb1, 0xBC),
		DeliveredDevonGoods:      flag(sb1, 0x95),
		DeliveredStevenLetter:    flag(sb1, 0xBD),
		ReceivedSurf:             flag(sb1, 0x7A),
		ReceivedDive:             flag(sb1, 0x7B),
		AquaEscapedInSubmarine:   flag(sb1, 0x70),
		DefeatedMagmaSpaceCenter: flag(sb1, 0x75),
		KyogreEscaped:            flag(sb1, 0x81),
		WallaceWentToSkyPillar:   flag(sb1, 0x137),
		MasterBallCollected:      flag(sb1, 0x464),
		RayquazaEncountered:      flag(sb1, 0x1C0),
	}
}

func orderIndex(order [4]uint8, target uint8) int {
	for i, value := range order {
		if value == target {
			return i
		}
	}
	return 0
}

var substructOrders = [24][4]uint8{
	{0, 1, 2, 3}, {0, 1, 3, 2}, {0, 2, 1, 3}, {0, 2, 3, 1}, {0, 3, 1, 2}, {0, 3, 2, 1},
	{1, 0, 2, 3}, {1, 0, 3, 2}, {1, 2, 0, 3}, {1, 2, 3, 0}, {1, 3, 0, 2}, {1, 3, 2, 0},
	{2, 0, 1, 3}, {2, 0, 3, 1}, {2, 1, 0, 3}, {2, 1, 3, 0}, {2, 3, 0, 1}, {2, 3, 1, 0},
	{3, 0, 1, 2}, {3, 0, 2, 1}, {3, 1, 0, 2}, {3, 1, 2, 0}, {3, 2, 0, 1}, {3, 2, 1, 0},
}

func decodeText(data []byte) string {
	runes := make([]rune, 0, len(data))
	for _, value := range data {
		if value == 0xFF {
			break
		}
		switch {
		case value == 0x00:
			runes = append(runes, ' ')
		case value >= 0xA1 && value <= 0xAA:
			runes = append(runes, rune('0'+value-0xA1))
		case value >= 0xBB && value <= 0xD4:
			runes = append(runes, rune('A'+value-0xBB))
		case value >= 0xD5 && value <= 0xEE:
			runes = append(runes, rune('a'+value-0xD5))
		case value == 0xAB:
			runes = append(runes, '!')
		case value == 0xAC:
			runes = append(runes, '?')
		case value == 0xAD:
			runes = append(runes, '.')
		case value == 0xAE:
			runes = append(runes, '-')
		case value == 0xB8:
			runes = append(runes, ',')
		default:
			runes = append(runes, '�')
		}
	}
	return string(runes)
}
