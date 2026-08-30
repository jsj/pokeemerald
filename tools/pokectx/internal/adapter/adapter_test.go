package adapter

import (
	"context"
	"fmt"
	"net"
	"testing"
)

func TestSocketReader(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		buffer := make([]byte, 32)
		_, _ = conn.Read(buffer)
		_, _ = fmt.Fprint(conn, `{"protocol_version":1,"game_code":"BPEE","observed_at":"2026-08-29T12:00:00Z"}`)
	}()
	observation, err := (SocketReader{Address: listener.Addr().String()}).Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if observation.GameCode != "BPEE" {
		t.Fatalf("game code = %q", observation.GameCode)
	}
}
