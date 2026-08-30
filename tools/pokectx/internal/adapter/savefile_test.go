package adapter

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveFileReaderSelectsNewestValidSlot(t *testing.T) {
	data := make([]byte, flashSize+16)
	writeTestSlot(data[:14*sectorSize], 7, 'A')
	writeTestSlot(data[14*sectorSize:28*sectorSize], 8, 'B')
	path := filepath.Join(t.TempDir(), "emerald.sav")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	observation, err := (SaveFileReader{Path: path}).Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if observation.SaveCounter != 8 || observation.Source != "save_file" {
		t.Fatalf("observation = %#v", observation)
	}
}

func TestSaveFileReaderRejectsCorruptSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.sav")
	if err := os.WriteFile(path, make([]byte, flashSize), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (SaveFileReader{Path: path}).Observe(context.Background()); err == nil {
		t.Fatal("expected validation error")
	}
}

func writeTestSlot(slot []byte, counter uint32, marker byte) {
	for physical := 0; physical < 14; physical++ {
		id := (physical + 5) % 14
		sector := slot[physical*sectorSize : (physical+1)*sectorSize]
		sector[0] = marker
		binary.LittleEndian.PutUint16(sector[0xFF4:0xFF6], uint16(id))
		binary.LittleEndian.PutUint16(sector[0xFF6:0xFF8], checksum(sector[:saveSectorDataSize(id)]))
		binary.LittleEndian.PutUint32(sector[0xFF8:0xFFC], sectorSignature)
		binary.LittleEndian.PutUint32(sector[0xFFC:0x1000], counter)
	}
}
