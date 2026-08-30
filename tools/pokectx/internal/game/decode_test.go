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
	copy(sb2[:8], []byte{0xC4, 0xBB, 0xC7, 0xBF, 0xCD, 0xFF}) // JAMES
	binary.LittleEndian.PutUint16(sb2[0x0E:0x10], 12)
	sb2[0x10], sb2[0x11] = 34, 56

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
	if result.Location.Name != "Mauville City" || result.Location.X != 18 || result.Location.Y != 7 {
		t.Fatalf("location = %#v", result.Location)
	}
	if len(result.Badges) != 1 || result.Badges[0] != "Stone Badge" {
		t.Fatalf("badges = %#v", result.Badges)
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

func setFlag(sb1 []byte, id uint16) { sb1[0x1270+int(id/8)] |= 1 << (id % 8) }
