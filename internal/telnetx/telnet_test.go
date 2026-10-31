package telnetx

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.b.Write(p) }
func (s *syncBuf) String() string              { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

// readUntil lê do cliente até achar want (ou falha em 2 s).
func readUntil(t *testing.T, c net.Conn, got *[]byte, want []byte) {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 256)
	for !bytes.Contains(*got, want) {
		n, err := c.Read(buf)
		*got = append(*got, buf[:n]...)
		if err != nil {
			t.Fatalf("esperava %q, chegou %q (%v)", want, *got, err)
		}
	}
}

func TestNegotiationAndAutoLogin(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	server := make(chan net.Conn, 1)
	go func() { c, _ := ln.Accept(); server <- c }()

	conn, err := Dial(context.Background(), Options{Address: "127.0.0.1", Port: port, Timeout: time.Second, Username: "noc", Password: "s3gr3do"})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	out := &syncBuf{}
	if err := conn.Start(100, 30, out); err != nil {
		t.Fatal(err)
	}
	s := <-server
	defer s.Close()
	var got []byte

	_, _ = s.Write([]byte{iac, do, optNAWS, iac, will, optEcho, iac, do, 99, iac, sb, optTType, ttypeSend, iac, se})
	readUntil(t, s, &got, []byte{iac, will, optNAWS})
	readUntil(t, s, &got, []byte{iac, sb, optNAWS, 0, 100, 0, 30, iac, se})
	readUntil(t, s, &got, []byte{iac, do, optEcho})
	readUntil(t, s, &got, []byte{iac, wont, 99})
	readUntil(t, s, &got, append(append([]byte{iac, sb, optTType, ttypeIs}, "xterm-256color"...), iac, se))

	_, _ = s.Write([]byte("Bem-vindo \xff\xff\r\nUsername: "))
	readUntil(t, s, &got, []byte("noc\r\x00"))
	_, _ = s.Write([]byte("Password: "))
	readUntil(t, s, &got, []byte("s3gr3do\r\x00"))

	// Senha errada: o equipamento pergunta de novo e o app não repete.
	got = got[:0]
	_, _ = s.Write([]byte("\r\nLogin failed\r\nUsername: "))
	_ = s.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if n, _ := s.Read(make([]byte, 64)); n != 0 {
		t.Fatal("app digitou de novo depois de falhar o login")
	}

	if err := conn.Resize(120, 40); err != nil {
		t.Fatal(err)
	}
	readUntil(t, s, &got, []byte{iac, sb, optNAWS, 0, 120, 0, 40, iac, se})

	if err := conn.Write([]byte("ls\r\xff")); err != nil {
		t.Fatal(err)
	}
	readUntil(t, s, &got, []byte("ls\r\x00\xff\xff"))

	time.Sleep(50 * time.Millisecond)
	if want := "Bem-vindo \xff\r\nUsername: Password: "; !bytes.HasPrefix([]byte(out.String()), []byte(want)) {
		t.Fatalf("saída: %q", out.String())
	}
	s.Close()
	if err := conn.Wait(); err != nil {
		t.Fatalf("fim normal virou erro: %v", err)
	}
}

func TestNAWSEscapesIAC(t *testing.T) {
	got := nawsFrame(255, 24)
	want := []byte{iac, sb, optNAWS, 0, iac, iac, 0, 24, iac, se}
	if !bytes.Equal(got, want) {
		t.Fatalf("%v", got)
	}
}

func TestEncodeKeepsCRLF(t *testing.T) {
	if got := encode([]byte("a\r\nb\r")); !bytes.Equal(got, []byte("a\r\nb\r\x00")) {
		t.Fatalf("%q", got)
	}
}

func TestNoAutoLoginWithoutPassword(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	server := make(chan net.Conn, 1)
	go func() { c, _ := ln.Accept(); server <- c }()
	conn, err := Dial(context.Background(), Options{Address: "127.0.0.1", Port: port, Timeout: time.Second, Username: "noc"})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.Start(80, 24, io.Discard)
	s := <-server
	defer s.Close()
	_, _ = s.Write([]byte("Username: "))
	_ = s.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if n, _ := s.Read(make([]byte, 64)); n != 0 {
		t.Fatal("sem senha não pode digitar nada")
	}
}
