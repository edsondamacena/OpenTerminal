package session

import (
	"sync"
	"time"
	"unicode/utf8"
)

const (
	FlushDelay = 8 * time.Millisecond
	FlushBytes = 32 * 1024
)

// Batcher junta a saída de uma sessão em lotes de até FlushDelay ou FlushBytes (research R10).
// Os lotes saem na ordem de chegada, sempre com texto UTF-8 inteiro.
type Batcher struct {
	emit func(string)

	// emitMu serializa as entregas (timer e Write) para manter a ordem.
	emitMu sync.Mutex
	mu     sync.Mutex
	buf    []byte
	dec    utf8Decoder
	timer  *time.Timer
	closed bool
}

func NewBatcher(emit func(string)) *Batcher {
	return &Batcher{emit: emit}
}

func (b *Batcher) Write(p []byte) (int, error) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return len(p), nil
	}
	b.buf = append(b.buf, b.dec.Decode(p)...)
	full := len(b.buf) >= FlushBytes
	if !full && b.timer == nil && len(b.buf) > 0 {
		b.timer = time.AfterFunc(FlushDelay, b.Flush)
	}
	b.mu.Unlock()
	if full {
		b.Flush()
	}
	return len(p), nil
}

// Flush entrega tudo o que está guardado, em lotes de no máximo FlushBytes.
func (b *Batcher) Flush() {
	b.emitMu.Lock()
	defer b.emitMu.Unlock()
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	data := b.take()
	b.mu.Unlock()
	b.send(data)
}

// Close entrega o que falta (um caractere partido vira U+FFFD) e desliga o Batcher: nada é
// emitido depois.
func (b *Batcher) Close() {
	b.emitMu.Lock()
	defer b.emitMu.Unlock()
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.buf = append(b.buf, b.dec.Flush()...)
	data := b.take()
	b.closed = true
	b.mu.Unlock()
	b.send(data)
}

func (b *Batcher) take() []byte {
	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}
	data := b.buf
	b.buf = nil
	return data
}

func (b *Batcher) send(data []byte) {
	for len(data) > 0 {
		cut := len(data)
		if cut > FlushBytes {
			cut = FlushBytes
			for cut > 0 && !utf8.RuneStart(data[cut]) {
				cut--
			}
		}
		b.emit(string(data[:cut]))
		data = data[cut:]
	}
}
