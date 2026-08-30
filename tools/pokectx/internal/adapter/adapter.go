package adapter

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"
)

type Observation struct {
	ProtocolVersion int        `json:"protocol_version"`
	GameCode        string     `json:"game_code"`
	ROMCRC32        string     `json:"rom_crc32"`
	ObservedAt      time.Time  `json:"observed_at"`
	SaveBlock1Hex   string     `json:"save_block_1_hex"`
	SaveBlock2Hex   string     `json:"save_block_2_hex"`
	PartyHex        string     `json:"party_hex"`
	PartyCount      uint8      `json:"party_count"`
	SaveCounter     uint32     `json:"save_counter"`
	Source          string     `json:"source,omitempty"`
	FileModifiedAt  *time.Time `json:"file_modified_at,omitempty"`
}

type Reader interface {
	Observe(context.Context) (Observation, error)
}

type SocketReader struct {
	Address string
	Timeout time.Duration
}

func (r SocketReader) Observe(ctx context.Context) (Observation, error) {
	var observation Observation
	timeout := r.Timeout
	if timeout == 0 {
		timeout = 3 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", r.Address)
	if err != nil {
		return observation, fmt.Errorf("connect to mGBA adapter at %s: %w", r.Address, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write([]byte("snapshot\n")); err != nil {
		return observation, fmt.Errorf("request snapshot: %w", err)
	}
	if err := json.NewDecoder(conn).Decode(&observation); err != nil {
		return observation, fmt.Errorf("decode adapter response: %w", err)
	}
	if observation.ProtocolVersion != 1 {
		return observation, fmt.Errorf("unsupported adapter protocol %d", observation.ProtocolVersion)
	}
	return observation, nil
}

type FixtureReader struct{ Path string }

func (r FixtureReader) Observe(_ context.Context) (Observation, error) {
	var observation Observation
	data, err := os.ReadFile(r.Path)
	if err != nil {
		return observation, fmt.Errorf("read fixture: %w", err)
	}
	if err := json.Unmarshal(data, &observation); err != nil {
		return observation, fmt.Errorf("decode fixture: %w", err)
	}
	return observation, nil
}

const (
	flashSize       = 32 * 4096
	sectorSize      = 4096
	sectorDataSize  = 3968
	sectorSignature = 0x08012025
	saveBlock1Size  = 0x3D88
	saveBlock2Size  = 0xF2C
)

// SaveFileReader reads a standard Emerald flash save without starting an emulator.
// Some emulators append private metadata. Pokectx validates only the first 128 KiB.
type SaveFileReader struct{ Path string }

func (r SaveFileReader) Observe(_ context.Context) (Observation, error) {
	var observation Observation
	data, err := os.ReadFile(r.Path)
	if err != nil {
		return observation, fmt.Errorf("read save file: %w", err)
	}
	if len(data) < flashSize {
		return observation, fmt.Errorf("save file is %d bytes; need at least %d", len(data), flashSize)
	}
	info, err := os.Stat(r.Path)
	if err != nil {
		return observation, fmt.Errorf("stat save file: %w", err)
	}
	sb1, sb2, counter, err := decodeSaveSlots(data[:flashSize])
	if err != nil {
		return observation, err
	}
	modified := info.ModTime().UTC()
	partyCount := sb1[0x234]
	if partyCount > 6 {
		return observation, fmt.Errorf("save file party count %d is invalid", partyCount)
	}
	partyEnd := 0x238 + int(partyCount)*100
	return Observation{
		ProtocolVersion: 1, GameCode: "BPEE", ObservedAt: modified,
		SaveBlock1Hex: hex.EncodeToString(sb1), SaveBlock2Hex: hex.EncodeToString(sb2),
		PartyHex: hex.EncodeToString(sb1[0x238:partyEnd]), PartyCount: partyCount,
		SaveCounter: counter, Source: "save_file", FileModifiedAt: &modified,
	}, nil
}

type decodedSlot struct {
	blocks  [14][]byte
	counter uint32
}

func decodeSaveSlots(data []byte) ([]byte, []byte, uint32, error) {
	var valid []decodedSlot
	for slot := 0; slot < 2; slot++ {
		decoded, ok := decodeSlot(data[slot*14*sectorSize : (slot+1)*14*sectorSize])
		if ok {
			valid = append(valid, decoded)
		}
	}
	if len(valid) == 0 {
		return nil, nil, 0, fmt.Errorf("save file has no complete slot with valid signatures and checksums")
	}
	selected := valid[0]
	if len(valid) == 2 && counterNewer(valid[1].counter, valid[0].counter) {
		selected = valid[1]
	}
	sb2 := append([]byte(nil), selected.blocks[0][:saveBlock2Size]...)
	sb1 := make([]byte, 0, saveBlock1Size)
	for id := 1; id <= 4; id++ {
		sb1 = append(sb1, selected.blocks[id]...)
	}
	return sb1[:saveBlock1Size], sb2, selected.counter, nil
}

func decodeSlot(data []byte) (decodedSlot, bool) {
	var result decodedSlot
	seen := uint16(0)
	for physical := 0; physical < 14; physical++ {
		sector := data[physical*sectorSize : (physical+1)*sectorSize]
		id := binary.LittleEndian.Uint16(sector[0xFF4:0xFF6])
		if id >= 14 || binary.LittleEndian.Uint32(sector[0xFF8:0xFFC]) != sectorSignature {
			return decodedSlot{}, false
		}
		size := saveSectorDataSize(int(id))
		if checksum(sector[:size]) != binary.LittleEndian.Uint16(sector[0xFF6:0xFF8]) {
			return decodedSlot{}, false
		}
		counter := binary.LittleEndian.Uint32(sector[0xFFC:0x1000])
		if seen != 0 && counter != result.counter {
			return decodedSlot{}, false
		}
		result.counter = counter
		result.blocks[id] = append([]byte(nil), sector[:size]...)
		seen |= 1 << id
	}
	return result, seen == (1<<14)-1
}

func saveSectorDataSize(id int) int {
	sizes := [14]int{saveBlock2Size, sectorDataSize, sectorDataSize, sectorDataSize, saveBlock1Size - 3*sectorDataSize,
		sectorDataSize, sectorDataSize, sectorDataSize, sectorDataSize, sectorDataSize, sectorDataSize, sectorDataSize, sectorDataSize, 0x83D0 - 8*sectorDataSize}
	return sizes[id]
}

func checksum(data []byte) uint16 {
	var sum uint32
	for offset := 0; offset+4 <= len(data); offset += 4 {
		sum += binary.LittleEndian.Uint32(data[offset : offset+4])
	}
	return uint16(sum) + uint16(sum>>16)
}

func counterNewer(candidate, current uint32) bool {
	return int32(candidate-current) > 0
}
