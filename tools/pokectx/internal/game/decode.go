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
	saveBlock1Size = 0x3D88
	saveBlock2Size = 0xF2C
	partyMonSize   = 100
	supportedCRC32 = "1f1c08fb"
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
	party, err := decodeBlock(observation.PartyHex, int(observation.PartyCount)*partyMonSize, "party")
	if err != nil {
		return model.Snapshot{}, err
	}
	if observation.GameCode != "BPEE" && observation.GameCode != "AGB-BPEE" {
		return model.Snapshot{}, fmt.Errorf("unsupported game code %q", observation.GameCode)
	}
	if observation.ROMCRC32 != "" && strings.ToLower(observation.ROMCRC32) != supportedCRC32 {
		return model.Snapshot{}, fmt.Errorf("unsupported Pokémon Emerald ROM CRC32 %q; expected %s", observation.ROMCRC32, supportedCRC32)
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
	snapshot := model.Snapshot{
		SchemaVersion: model.SchemaVersion,
		GameCode:      observation.GameCode,
		ROMCRC32:      observation.ROMCRC32,
		PlayerName:    playerName,
		Location:      model.Location{MapGroup: group, MapNum: number, MapID: mapID, Name: mapName, X: int(int16(binary.LittleEndian.Uint16(sb1[0:2]))), Y: int(int16(binary.LittleEndian.Uint16(sb1[2:4])))},
		Party:         d.decodeParty(party, int(observation.PartyCount)),
		Badges:        decodeBadges(sb1),
		KeyItems:      d.decodeKeyItems(sb1, binary.LittleEndian.Uint32(sb2[0xAC:0xB0])),
		Save:          model.SaveEvidence{Counter: observation.SaveCounter, PlayTime: fmt.Sprintf("%d:%02d:%02d", binary.LittleEndian.Uint16(sb2[0x0E:0x10]), sb2[0x10], sb2[0x11]), Source: saveSource, FileModifiedAt: observation.FileModifiedAt},
		Evidence:      model.Evidence{Source: source, ObservedAt: observation.ObservedAt, Freshness: freshness(observation.ObservedAt)},
	}
	snapshot.Milestones, snapshot.Next = progress(sb1)
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

func (d Decoder) decodeParty(data []byte, count int) []model.Pokemon {
	if count > 6 {
		count = 6
	}
	party := make([]model.Pokemon, 0, count)
	for i := 0; i < count; i++ {
		mon := data[i*partyMonSize : (i+1)*partyMonSize]
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
		moveIDs := make([]uint16, 0, 4)
		moveNames := make([]string, 0, 4)
		for move := 0; move < 4; move++ {
			id := binary.LittleEndian.Uint16(attacks[move*2 : move*2+2])
			if id == 0 {
				continue
			}
			moveIDs = append(moveIDs, id)
			moveNames = append(moveNames, d.Catalog.MoveName(id))
		}
		party = append(party, model.Pokemon{Slot: i + 1, SpeciesID: speciesID, Species: d.Catalog.SpeciesName(speciesID), Nickname: decodeText(mon[8:18]), Level: mon[84], HP: binary.LittleEndian.Uint16(mon[86:88]), MaxHP: binary.LittleEndian.Uint16(mon[88:90]), MoveIDs: moveIDs, Moves: moveNames, IsEgg: mon[19]&(1<<2) != 0})
	}
	return party
}

func (d Decoder) decodeKeyItems(sb1 []byte, encryptionKey uint32) []model.Item {
	const offset, count = 0x5D8, 30
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

func progress(sb1 []byte) ([]string, model.Objective) {
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
