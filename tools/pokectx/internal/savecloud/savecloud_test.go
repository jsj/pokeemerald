package savecloud

import (
	"context"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportAndImport(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.sav")
	if err := os.WriteFile(source, testSave(7), 0o600); err != nil {
		t.Fatal(err)
	}
	exported := filepath.Join(root, "exported.sav")
	result, err := Export(context.Background(), source, exported)
	if err != nil || result.Action != "exported" {
		t.Fatalf("Export() = %#v, %v", result, err)
	}

	destination := filepath.Join(root, "active", "Pokemon Emerald.sav")
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("old save"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err = Import(context.Background(), exported, destination, "127.0.0.1:1")
	if err != nil || result.Action != "imported" || result.BackupPath == "" {
		t.Fatalf("Import() = %#v, %v", result, err)
	}
	if _, err := ReadValid(context.Background(), destination); err != nil {
		t.Fatalf("imported save is invalid: %v", err)
	}
	backup, err := os.ReadFile(result.BackupPath)
	if err != nil || string(backup) != "old save" {
		t.Fatalf("backup = %q, %v", backup, err)
	}
}

func TestImportRefusesWhileAdapterIsRunning(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	path := filepath.Join(t.TempDir(), "source.sav")
	if err := os.WriteFile(path, testSave(1), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Import(context.Background(), path, filepath.Join(t.TempDir(), "active.sav"), listener.Addr().String())
	if err == nil || !strings.Contains(err.Error(), "close mGBA") {
		t.Fatalf("Import() error = %v", err)
	}
}

func testSave(counter uint32) []byte {
	data := make([]byte, 32*4096)
	for slot := 0; slot < 2; slot++ {
		for id := 0; id < 14; id++ {
			sector := data[(slot*14+id)*4096 : (slot*14+id+1)*4096]
			binary.LittleEndian.PutUint16(sector[0xFF4:0xFF6], uint16(id))
			binary.LittleEndian.PutUint16(sector[0xFF6:0xFF8], 0)
			binary.LittleEndian.PutUint32(sector[0xFF8:0xFFC], 0x08012025)
			binary.LittleEndian.PutUint32(sector[0xFFC:0x1000], counter+uint32(slot))
		}
	}
	return data
}
