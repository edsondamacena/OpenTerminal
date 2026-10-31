package sftpx

import (
	"bytes"
	"io"
	"testing"
)

func TestProgressReaderPreservesSizeForConcurrentUpload(t *testing.T) {
	content := []byte("conteúdo")
	var progressed int64
	reader := &progressReader{
		reader: bytes.NewReader(content),
		size:   int64(len(content)),
		update: func(n int64) { progressed += n },
	}
	data, err := io.ReadAll(reader)
	if err != nil || string(data) != "conteúdo" {
		t.Fatalf("leitura = %q, %v", data, err)
	}
	if reader.Size() != int64(len(content)) || progressed != int64(len(data)) {
		t.Fatalf("size = %d, progresso = %d", reader.Size(), progressed)
	}
}
