//go:build integration

package sftpx

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	sftpTestAddress = "127.0.0.1:2222"
	sftpTestSize    = int64(100 * 1024 * 1024)
)

type transferRecorder struct {
	mu        sync.Mutex
	transfers map[string]Transfer
	conflicts []ConflictEvent
}

func newTransferRecorder() (*transferRecorder, *TransferManager) {
	recorder := &transferRecorder{transfers: make(map[string]Transfer)}
	manager := NewTransferManager(TransferEvents{
		Transfer: func(transfer Transfer) {
			recorder.mu.Lock()
			recorder.transfers[transfer.ID] = transfer
			recorder.mu.Unlock()
		},
		Conflict: func(conflict ConflictEvent) {
			recorder.mu.Lock()
			recorder.conflicts = append(recorder.conflicts, conflict)
			recorder.mu.Unlock()
		},
	}, TransferManagerOptions{})
	return recorder, manager
}

func (r *transferRecorder) transfer(id string) (Transfer, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	transfer, ok := r.transfers[id]
	return transfer, ok
}

func (r *transferRecorder) conflict(index int) (ConflictEvent, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if index < 0 || index >= len(r.conflicts) {
		return ConflictEvent{}, false
	}
	return r.conflicts[index], true
}

func waitTransfer(t *testing.T, recorder *transferRecorder, id string, timeout time.Duration) Transfer {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if transfer, ok := recorder.transfer(id); ok && isFinalTransferState(transfer.State) {
			return transfer
		}
		time.Sleep(50 * time.Millisecond)
	}
	transfer, _ := recorder.transfer(id)
	t.Fatalf("tempo esgotado esperando transferência %q (estado %q)", id, transfer.State)
	return Transfer{}
}

func waitUntil(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("tempo esgotado esperando condição SFTP")
}

func isFinalTransferState(state TransferState) bool {
	return state == TransferCompleted || state == TransferCancelled || state == TransferFailed || state == TransferSkipped
}

func newIntegrationEndpoint(t *testing.T) (*Endpoint, string) {
	t.Helper()
	client, err := ssh.Dial("tcp", sftpTestAddress, &ssh.ClientConfig{
		User:            "pwuser",
		Auth:            []ssh.AuthMethod{ssh.Password("pw-test-123")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Servidor efêmero exclusivo da fixture.
		Timeout:         10 * time.Second,
	})
	if err != nil {
		t.Fatalf("conectar ao sshd de teste (rode node scripts/sshd.mjs up): %v", err)
	}
	endpoint, err := NewEndpoint("sftp-test", client)
	if err != nil {
		_ = client.Close()
		t.Fatal(err)
	}
	home, err := endpoint.RealPath(".")
	if err != nil {
		_ = endpoint.Close()
		_ = client.Close()
		t.Fatal(err)
	}
	work := fmt.Sprintf("%s/it-%d", home, time.Now().UnixNano())
	if err := endpoint.Mkdir(work); err != nil {
		_ = endpoint.Close()
		_ = client.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = endpoint.Remove(work, true)
		_ = endpoint.Close()
		_ = client.Close()
	})
	return endpoint, work
}

func randomFile(t *testing.T, dir, name string, size int64) string {
	t.Helper()
	path := filepath.Join(dir, name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 4*1024*1024)
	for written := int64(0); written < size; {
		chunk := int64(len(buffer))
		if remaining := size - written; remaining < chunk {
			chunk = remaining
		}
		if _, err := rand.Read(buffer[:chunk]); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if _, err := file.Write(buffer[:chunk]); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		written += chunk
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func fileSHA256(t *testing.T, path string) [sha256.Size]byte {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		t.Fatal(err)
	}
	var sum [sha256.Size]byte
	copy(sum[:], hash.Sum(nil))
	return sum
}

func TestSFTPIntegrationBrowserOperations(t *testing.T) {
	endpoint, work := newIntegrationEndpoint(t)
	dir := work + "/teste ç ã"
	if err := endpoint.Mkdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := endpoint.Mkdir(dir + "/sub"); err != nil {
		t.Fatal(err)
	}
	local := t.TempDir()
	file := filepath.Join(local, "a.txt")
	if err := os.WriteFile(file, []byte("olá"), 0o600); err != nil {
		t.Fatal(err)
	}
	recorder, manager := newTransferRecorder()
	ids, err := manager.Upload(endpoint, []string{file}, dir+"/sub")
	if err != nil || len(ids) != 1 {
		t.Fatalf("upload: ids=%v err=%v", ids, err)
	}
	if transfer := waitTransfer(t, recorder, ids[0], 30*time.Second); transfer.State != TransferCompleted {
		t.Fatalf("upload terminou em %q: %v", transfer.State, transfer.Reason)
	}

	entries, err := endpoint.List(work)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "teste ç ã" || entries[0].Type != EntryDirectory || entries[0].Path != dir {
		t.Fatalf("listagem da pasta: %+v", entries)
	}
	if entries[0].ModifiedAt.IsZero() {
		t.Fatal("data de modificação não preenchida")
	}
	sub, err := endpoint.List(dir + "/sub")
	if err != nil {
		t.Fatal(err)
	}
	if len(sub) != 1 || sub[0].Name != "a.txt" || sub[0].Type != EntryFile || sub[0].Size != 4 {
		t.Fatalf("listagem do arquivo: %+v", sub)
	}

	renamed := work + "/renomeada"
	if err := endpoint.Rename(dir, renamed); err != nil {
		t.Fatal(err)
	}
	entries, err = endpoint.List(work)
	if err != nil || len(entries) != 1 || entries[0].Name != "renomeada" {
		t.Fatalf("depois de renomear: %+v err=%v", entries, err)
	}
	if err := endpoint.Remove(renamed, false); err == nil {
		t.Fatal("remoção não recursiva de pasta preenchida deveria falhar")
	}
	if err := endpoint.Remove(renamed, true); err != nil {
		t.Fatal(err)
	}
	entries, err = endpoint.List(work)
	if err != nil || len(entries) != 0 {
		t.Fatalf("pasta deveria estar vazia: %+v err=%v", entries, err)
	}
}

func TestSFTPIntegrationListsDirectoriesFirstByName(t *testing.T) {
	endpoint, work := newIntegrationEndpoint(t)
	dir := work + "/ordem"
	if err := endpoint.Mkdir(dir); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "z.txt")
	if err := os.WriteFile(file, []byte("z"), 0o600); err != nil {
		t.Fatal(err)
	}
	recorder, manager := newTransferRecorder()
	ids, err := manager.Upload(endpoint, []string{file}, dir)
	if err != nil || len(ids) != 1 {
		t.Fatalf("upload: ids=%v err=%v", ids, err)
	}
	if transfer := waitTransfer(t, recorder, ids[0], 30*time.Second); transfer.State != TransferCompleted {
		t.Fatalf("upload terminou em %q: %v", transfer.State, transfer.Reason)
	}
	if err := endpoint.Mkdir(dir + "/b"); err != nil {
		t.Fatal(err)
	}
	if err := endpoint.Mkdir(dir + "/A"); err != nil {
		t.Fatal(err)
	}
	entries, err := endpoint.List(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name
	}
	if want := []string{"A", "b", "z.txt"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("ordem = %v, esperado %v", names, want)
	}
}

func TestSFTPIntegrationRoundTrip100MB(t *testing.T) {
	endpoint, work := newIntegrationEndpoint(t)
	source := randomFile(t, t.TempDir(), "grande.bin", sftpTestSize)
	recorder, manager := newTransferRecorder()
	uploadIDs, err := manager.Upload(endpoint, []string{source}, work)
	if err != nil || len(uploadIDs) != 1 {
		t.Fatalf("upload: ids=%v err=%v", uploadIDs, err)
	}
	upload := waitTransfer(t, recorder, uploadIDs[0], 180*time.Second)
	if upload.State != TransferCompleted || upload.BytesTotal != sftpTestSize || upload.BytesDone != sftpTestSize {
		t.Fatalf("upload incompleto: %+v", upload)
	}

	destination := t.TempDir()
	downloadIDs, err := manager.Download(endpoint, []string{work + "/grande.bin"}, destination)
	if err != nil || len(downloadIDs) != 1 {
		t.Fatalf("download: ids=%v err=%v", downloadIDs, err)
	}
	download := waitTransfer(t, recorder, downloadIDs[0], 180*time.Second)
	if download.State != TransferCompleted || download.SessionID != "sftp-test" {
		t.Fatalf("download incompleto: %+v", download)
	}
	if got, want := fileSHA256(t, filepath.Join(destination, "grande.bin")), fileSHA256(t, source); got != want {
		t.Fatalf("SHA-256 do download difere: %x != %x", got, want)
	}
}

func TestSFTPIntegrationConflictSkipAndReplace(t *testing.T) {
	endpoint, work := newIntegrationEndpoint(t)
	dir := work + "/conflito"
	if err := endpoint.Mkdir(dir); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "c.txt")
	if err := os.WriteFile(file, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	recorder, manager := newTransferRecorder()
	first, err := manager.Upload(endpoint, []string{file}, dir)
	if err != nil || len(first) != 1 {
		t.Fatalf("primeiro upload: ids=%v err=%v", first, err)
	}
	if transfer := waitTransfer(t, recorder, first[0], 30*time.Second); transfer.State != TransferCompleted {
		t.Fatalf("primeiro upload terminou em %q", transfer.State)
	}

	if err := os.WriteFile(file, []byte("novo conteúdo"), 0o600); err != nil {
		t.Fatal(err)
	}
	skip, err := manager.Upload(endpoint, []string{file}, dir)
	if err != nil || len(skip) != 1 {
		t.Fatalf("upload com conflito: ids=%v err=%v", skip, err)
	}
	waitUntil(t, 30*time.Second, func() bool {
		conflict, ok := recorder.conflict(0)
		return ok && conflict.TransferID == skip[0] && conflict.Name == "c.txt" && conflict.ExistingSize == 8
	})
	if err := manager.ResolveConflict(skip[0], ConflictSkip); err != nil {
		t.Fatal(err)
	}
	if transfer := waitTransfer(t, recorder, skip[0], 30*time.Second); transfer.State != TransferSkipped {
		t.Fatalf("skip terminou em %q", transfer.State)
	}

	downloadDir := t.TempDir()
	download, err := manager.Download(endpoint, []string{dir + "/c.txt"}, downloadDir)
	if err != nil || len(download) != 1 {
		t.Fatalf("download: ids=%v err=%v", download, err)
	}
	if transfer := waitTransfer(t, recorder, download[0], 30*time.Second); transfer.State != TransferCompleted {
		t.Fatalf("download terminou em %q", transfer.State)
	}
	contents, err := os.ReadFile(filepath.Join(downloadDir, "c.txt"))
	if err != nil || string(contents) != "original" {
		t.Fatalf("conteúdo depois de pular = %q, err=%v", contents, err)
	}

	replace, err := manager.Upload(endpoint, []string{file}, dir)
	if err != nil || len(replace) != 1 {
		t.Fatalf("substituição: ids=%v err=%v", replace, err)
	}
	waitUntil(t, 30*time.Second, func() bool { _, ok := recorder.conflict(1); return ok })
	if err := manager.ResolveConflict(replace[0], ConflictReplace); err != nil {
		t.Fatal(err)
	}
	if transfer := waitTransfer(t, recorder, replace[0], 30*time.Second); transfer.State != TransferCompleted {
		t.Fatalf("substituição terminou em %q", transfer.State)
	}

	again, err := manager.Download(endpoint, []string{dir + "/c.txt"}, downloadDir)
	if err != nil || len(again) != 1 {
		t.Fatalf("segundo download: ids=%v err=%v", again, err)
	}
	waitUntil(t, 30*time.Second, func() bool { _, ok := recorder.conflict(2); return ok })
	if err := manager.ResolveConflict(again[0], ConflictReplace); err != nil {
		t.Fatal(err)
	}
	if transfer := waitTransfer(t, recorder, again[0], 30*time.Second); transfer.State != TransferCompleted {
		t.Fatalf("download com substituição terminou em %q", transfer.State)
	}
	contents, err = os.ReadFile(filepath.Join(downloadDir, "c.txt"))
	if err != nil || string(contents) != "novo conteúdo" {
		t.Fatalf("conteúdo substituído = %q, err=%v", contents, err)
	}
}

func TestSFTPIntegrationCancelLeavesNoPartial(t *testing.T) {
	endpoint, work := newIntegrationEndpoint(t)
	dir := work + "/cancelar"
	if err := endpoint.Mkdir(dir); err != nil {
		t.Fatal(err)
	}
	source := randomFile(t, t.TempDir(), "cancela.bin", sftpTestSize)
	recorder, manager := newTransferRecorder()
	uploads, err := manager.Upload(endpoint, []string{source}, dir)
	if err != nil || len(uploads) != 1 {
		t.Fatalf("upload: ids=%v err=%v", uploads, err)
	}
	waitUntil(t, 30*time.Second, func() bool {
		transfer, ok := recorder.transfer(uploads[0])
		return ok && transfer.State == TransferRunning && transfer.BytesDone > 0
	})
	if err := manager.Cancel(uploads[0]); err != nil {
		t.Fatal(err)
	}
	if transfer := waitTransfer(t, recorder, uploads[0], 120*time.Second); transfer.State != TransferCancelled {
		t.Fatalf("upload cancelado terminou em %q", transfer.State)
	}
	entries, err := endpoint.List(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("upload cancelado deixou arquivos: %+v err=%v", entries, err)
	}

	uploads, err = manager.Upload(endpoint, []string{source}, dir)
	if err != nil || len(uploads) != 1 {
		t.Fatalf("upload para cancelamento do download: ids=%v err=%v", uploads, err)
	}
	if transfer := waitTransfer(t, recorder, uploads[0], 180*time.Second); transfer.State != TransferCompleted {
		t.Fatalf("upload preparatório terminou em %q", transfer.State)
	}
	destination := t.TempDir()
	downloads, err := manager.Download(endpoint, []string{dir + "/cancela.bin"}, destination)
	if err != nil || len(downloads) != 1 {
		t.Fatalf("download: ids=%v err=%v", downloads, err)
	}
	waitUntil(t, 30*time.Second, func() bool {
		transfer, ok := recorder.transfer(downloads[0])
		return ok && transfer.State == TransferRunning && transfer.BytesDone > 0
	})
	if err := manager.Cancel(downloads[0]); err != nil {
		t.Fatal(err)
	}
	if transfer := waitTransfer(t, recorder, downloads[0], 120*time.Second); transfer.State != TransferCancelled {
		t.Fatalf("download cancelado terminou em %q", transfer.State)
	}
	entriesLocal, err := os.ReadDir(destination)
	if err != nil || len(entriesLocal) != 0 {
		t.Fatalf("download cancelado deixou arquivos: %v err=%v", entriesLocal, err)
	}
}
