package adapter

import (
	"bufio"
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

func TestExecuteCheat(t *testing.T) {
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
		line, _ := bufio.NewReader(conn).ReadString('\n')
		if line != "cheat 3 83 10 42 -\n" {
			return
		}
		_, _ = fmt.Fprint(conn, `{"protocol_version":1,"nonce":42,"status":2,"result":10}`)
	}()
	response, err := (SocketReader{Address: listener.Addr().String()}).ExecuteCheat(
		t.Context(), CheatRequest{Command: 3, Arg1: 83, Arg2: 10, Nonce: 42},
	)
	if err != nil {
		t.Fatal(err)
	}
	if response.Result != 10 {
		t.Fatalf("result = %d", response.Result)
	}
}

func TestExecuteCheatSendsData(t *testing.T) {
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
		line, _ := bufio.NewReader(conn).ReadString('\n')
		if line != "cheat 13 0 0 42 bbccff\n" {
			return
		}
		_, _ = fmt.Fprint(conn, `{"protocol_version":1,"nonce":42,"status":2,"result":0}`)
	}()
	_, err = (SocketReader{Address: listener.Addr().String()}).ExecuteCheat(
		t.Context(), CheatRequest{Command: 13, Nonce: 42, Data: []byte{0xBB, 0xCC, 0xFF}},
	)
	if err != nil {
		t.Fatal(err)
	}
}
