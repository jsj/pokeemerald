package game

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"

	"github.com/jsj/pokectx/internal/adapter"
	"github.com/jsj/pokectx/internal/catalog"
)

func TestDecodeSnapshot(t *testing.T) {
	sb1 := make([]byte, saveBlock1Size)
	sb2 := make([]byte, saveBlock2Size)
	sb1[4], sb1[5] = 0, 2
	binary.LittleEndian.PutUint16(sb1[0:2], 18)
	binary.LittleEndian.PutUint16(sb1[2:4], 7)
	setFlag(sb1, 0x4F0)
	setFlag(sb1, 0x867)
	setFlag(sb1, 0x52)
	setFlag(sb1, 0x74)
	setFlag(sb1, 0x82)
	setFlag(sb1, 0x8F)
	setFlag(sb1, 0x90)
	setFlag(sb1, 0xBC)
	setVariable(sb1, 0x4084, 3)
	setVariable(sb1, 0x4085, 2)
	copy(sb2[:8], []byte{0xC4, 0xBB, 0xC7, 0xBF, 0xCD, 0xFF}) // JAMES
	binary.LittleEndian.PutUint16(sb2[0x0E:0x10], 12)
	sb2[0x10], sb2[0x11] = 34, 56
	binary.LittleEndian.PutUint32(sb2[0xAC:0xB0], 0x12345678)
	binary.LittleEndian.PutUint32(sb1[0x490:0x494], 1746^0x12345678)
	binary.LittleEndian.PutUint16(sb1[0x560:0x562], 13) // Potion
	binary.LittleEndian.PutUint16(sb1[0x562:0x564], 4^0x5678)
	setEncryptedItem(sb1, 0x650, 4, 7, 0x5678)   // Poké Ball
	setEncryptedItem(sb1, 0x690, 327, 1, 0x5678) // TM39
	setEncryptedItem(sb1, 0x790, 139, 2, 0x5678) // Oran Berry
	setVariable(sb1, 0x4021, 83)

	result, err := (Decoder{Catalog: catalog.Default}).Decode(adapter.Observation{
		ProtocolVersion: 1,
		GameCode:        "BPEE",
		ROMCRC32:        "1f1c08fb",
		ObservedAt:      time.Now(),
		SaveBlock1Hex:   hex.EncodeToString(sb1),
		SaveBlock2Hex:   hex.EncodeToString(sb2),
		PartyHex:        "",
		PartyCount:      0,
		SaveCounter:     9,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.PlayerName != "JAMES" {
		t.Fatalf("player name = %q", result.PlayerName)
	}
	if result.Money != 1746 {
		t.Fatalf("money = %d", result.Money)
	}
	if result.Location.Name != "Mauville City" || result.Location.X != 18 || result.Location.Y != 7 {
		t.Fatalf("location = %#v", result.Location)
	}
	if len(result.Badges) != 1 || result.Badges[0] != "Stone Badge" {
		t.Fatalf("badges = %#v", result.Badges)
	}
	if len(result.Items) != 1 || result.Items[0].Name != "Potion" || result.Items[0].Quantity != 4 {
		t.Fatalf("items = %#v", result.Items)
	}
	if len(result.PokeBalls) != 1 || result.PokeBalls[0].Quantity != 7 {
		t.Fatalf("poke balls = %#v", result.PokeBalls)
	}
	if len(result.TMsAndHMs) != 1 || result.TMsAndHMs[0].Name != "Tm39" {
		t.Fatalf("TMs and HMs = %#v", result.TMsAndHMs)
	}
	if len(result.Berries) != 1 || result.Berries[0].Name != "Oran Berry" {
		t.Fatalf("berries = %#v", result.Berries)
	}
	if result.RepelSteps != 83 {
		t.Fatalf("repel steps = %d", result.RepelSteps)
	}
	if result.Next.Destination != "Dewford Town Gym" {
		t.Fatalf("next = %#v", result.Next)
	}
	if result.Save.Counter != 9 || result.Save.PlayTime != "12:34:56" {
		t.Fatalf("save = %#v", result.Save)
	}
}

func TestDecodeRejectsWrongGame(t *testing.T) {
	block1 := make([]byte, saveBlock1Size)
	block2 := make([]byte, saveBlock2Size)
	_, err := (Decoder{Catalog: catalog.Default}).Decode(adapter.Observation{GameCode: "AGB-AXVE", SaveBlock1Hex: hex.EncodeToString(block1), SaveBlock2Hex: hex.EncodeToString(block2)})
	if err == nil {
		t.Fatal("expected unsupported game error")
	}
}

func TestDecodeRejectsModifiedEmeraldROM(t *testing.T) {
	block1 := make([]byte, saveBlock1Size)
	block2 := make([]byte, saveBlock2Size)
	_, err := (Decoder{Catalog: catalog.Default}).Decode(adapter.Observation{GameCode: "BPEE", ROMCRC32: "deadbeef", SaveBlock1Hex: hex.EncodeToString(block1), SaveBlock2Hex: hex.EncodeToString(block2)})
	if err == nil {
		t.Fatal("expected unsupported ROM revision error")
	}
}

func TestDecodeStatus(t *testing.T) {
	tests := map[uint32]string{
		0:    "healthy",
		0x3:  "sleep",
		0x8:  "poison",
		0x10: "burn",
		0x20: "freeze",
		0x40: "paralysis",
		0x80: "toxic_poison",
	}
	for status, want := range tests {
		if got := decodeStatus(status); got != want {
			t.Errorf("decodeStatus(%#x) = %q, want %q", status, got, want)
		}
	}
}

func TestIsShinyUsesTrainerAndPersonalityValues(t *testing.T) {
	if !isShiny(0x12345678, 0x444c0000) {
		t.Fatal("expected shiny personality")
	}
	if isShiny(0x12345678, 0x12346678) {
		t.Fatal("expected normal personality")
	}
}

func TestDecodeTrainerBattle(t *testing.T) {
	const battlePokemonSize = 0x58
	battleData := make([]byte, battlePokemonSize*2)
	opponent := battleData[battlePokemonSize:]
	binary.LittleEndian.PutUint16(opponent[0:2], 286) // Poochyena
	binary.LittleEndian.PutUint16(opponent[0x0C:0x0E], 33)
	for index := 0; index < 8; index++ {
		opponent[0x18+index] = 6
	}
	opponent[0x18+6] = 2 // Accuracy stage -4.
	opponent[0x21], opponent[0x22] = 17, 17
	opponent[0x24] = 31
	binary.LittleEndian.PutUint16(opponent[0x28:0x2A], 12)
	opponent[0x2A] = 11
	binary.LittleEndian.PutUint16(opponent[0x2C:0x2E], 30)

	battle, err := (Decoder{Catalog: catalog.Default}).decodeBattle(adapter.Observation{
		BattleTypeFlags: 1 << 3,
		BattlersCount:   2,
		BattleMonsHex:   hex.EncodeToString(battleData),
	})
	if err != nil {
		t.Fatal(err)
	}
	if battle.Kind != "trainer" || battle.CanAttemptRun {
		t.Fatalf("battle = %#v", battle)
	}
	if got := battle.Pokemon[1]; got.Species != "Poochyena" || got.HP != 12 || got.StatStages["accuracy"] != -4 {
		t.Fatalf("opponent = %#v", got)
	}
}

func TestProgressStartsWithBirchInsteadOfRoxanne(t *testing.T) {
	saveBlock := make([]byte, saveBlock1Size)
	_, next := progress(saveBlock, decodeStory(saveBlock))
	if next.ID != "rescue_birch" {
		t.Fatalf("next = %#v", next)
	}
}

func setFlag(sb1 []byte, id uint16) { sb1[0x1270+int(id/8)] |= 1 << (id % 8) }

func setVariable(sb1 []byte, id, value uint16) {
	offset := 0x139C + int(id-0x4000)*2
	binary.LittleEndian.PutUint16(sb1[offset:offset+2], value)
}

func setEncryptedItem(sb1 []byte, offset int, id, quantity, encryptionKey uint16) {
	binary.LittleEndian.PutUint16(sb1[offset:offset+2], id)
	binary.LittleEndian.PutUint16(sb1[offset+2:offset+4], quantity^encryptionKey)
}
