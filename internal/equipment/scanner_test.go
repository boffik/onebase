package equipment

import (
	"context"
	"net"
	"testing"
)

func TestScanner_Stream(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		conn.Write([]byte("12345\n67890\n"))
		conn.Close() // EOF завершает Stream
	}()

	dev, err := Open("scanner_tcp", map[string]string{"порт": ln.Addr().String()})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer dev.Disconnect()
	src, ok := dev.(EventSource)
	if !ok {
		t.Fatal("устройство не реализует EventSource")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var codes []string
	if err := src.Stream(ctx, func(c string) { codes = append(codes, c) }); err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if len(codes) != 2 || codes[0] != "12345" || codes[1] != "67890" {
		t.Errorf("коды = %v, ожидались [12345 67890]", codes)
	}
}
