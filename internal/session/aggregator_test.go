package session

import (
	"bytes"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

type sink struct {
	mu     sync.Mutex
	chunks []string
	at     []time.Time
}

func (s *sink) emit(data string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chunks = append(s.chunks, data)
	s.at = append(s.at, time.Now())
}

func (s *sink) joined() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Join(s.chunks, "")
}

func (s *sink) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.chunks)
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("tempo esgotado")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestBatcherFlushesAfterDelay(t *testing.T) {
	var s sink
	b := NewBatcher(s.emit)
	defer b.Close()
	start := time.Now()
	_, _ = b.Write([]byte("abc"))
	_, _ = b.Write([]byte("def"))
	waitFor(t, func() bool { return s.count() == 1 })
	if got := s.joined(); got != "abcdef" {
		t.Fatalf("lote = %q", got)
	}
	if d := s.at[0].Sub(start); d < FlushDelay || d > FlushDelay+200*time.Millisecond {
		t.Fatalf("lote saiu em %v", d)
	}
}

func TestBatcherFlushesAtSizeLimit(t *testing.T) {
	var s sink
	b := NewBatcher(s.emit)
	defer b.Close()
	_, _ = b.Write(bytes.Repeat([]byte("x"), FlushBytes+10))
	// Passou do limite: sai na hora, sem esperar o timer, em lotes de no máximo FlushBytes.
	if s.count() != 2 || len(s.chunks[0]) != FlushBytes || len(s.chunks[1]) != 10 {
		t.Fatalf("lotes = %d", s.count())
	}
}

func TestBatcherTenMiBInOrder(t *testing.T) {
	var s sink
	b := NewBatcher(s.emit)
	unit := "linha 123 ação ✓ 😀 fim\r\n"
	var want strings.Builder
	for want.Len() < 10<<20 {
		want.WriteString(unit)
	}
	raw := []byte(want.String())
	rng := rand.New(rand.NewSource(1))
	for len(raw) > 0 {
		n := min(1+rng.Intn(5000), len(raw))
		_, _ = b.Write(raw[:n])
		raw = raw[n:]
	}
	b.Close()
	if got := s.joined(); got != want.String() {
		t.Fatalf("saída difere: %d bytes, esperado %d", len(got), want.Len())
	}
	for i, c := range s.chunks {
		if len(c) > FlushBytes {
			t.Fatalf("lote %d com %d bytes", i, len(c))
		}
		if !utf8.ValidString(c) {
			t.Fatalf("lote %d com UTF-8 partido", i)
		}
	}
}

func TestBatcherNoEmitAfterClose(t *testing.T) {
	var s sink
	b := NewBatcher(s.emit)
	_, _ = b.Write([]byte("a"))
	b.Close()
	n := s.count()
	_, _ = b.Write([]byte("b"))
	time.Sleep(3 * FlushDelay)
	if s.count() != n || s.joined() != "a" {
		t.Fatalf("emitiu depois de fechar: %q", s.joined())
	}
}

func TestDecoderSplitRune(t *testing.T) {
	var d utf8Decoder
	euro := []byte("€") // 3 bytes
	got := d.Decode([]byte{'a', euro[0]}) + d.Decode(euro[1:2]) + d.Decode(append(euro[2:], 'b'))
	if got != "a€b" {
		t.Fatalf("got %q", got)
	}
	emoji := []byte("😀")
	var parts []string
	for _, c := range emoji {
		parts = append(parts, d.Decode([]byte{c}))
	}
	if strings.Join(parts, "") != "😀" || parts[0] != "" {
		t.Fatalf("parts %q", parts)
	}
}

func TestDecoderInvalidBytes(t *testing.T) {
	var d utf8Decoder
	if got := d.Decode([]byte{'a', 0xff, 'b', 0xc3, 'c'}); got != "a�b�c" {
		t.Fatalf("got %q", got)
	}
	// Prefixo incompleto no fim só vira U+FFFD no Flush.
	if got := d.Decode([]byte{'x', 0xe2, 0x82}); got != "x" {
		t.Fatalf("got %q", got)
	}
	if got := d.Flush(); got != "�" {
		t.Fatalf("flush %q", got)
	}
}

func TestBatcherFlushesPartialRuneOnClose(t *testing.T) {
	var s sink
	b := NewBatcher(s.emit)
	_, _ = b.Write([]byte{'o', 'k', 0xe2})
	b.Close()
	if got := s.joined(); got != "ok�" {
		t.Fatalf("got %q", got)
	}
}
