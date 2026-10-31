// O benchmark compara o gerenciador SFTP do OpenTerminal com o sftp.exe do Windows (SC-006).
// Uso: go run ./scripts/bench-sftp [MB] (padrão: 1024; exige o sshd de teste em 127.0.0.1:2222).
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"openterminal/internal/sftpx"
)

const (
	megabyte = int64(1024 * 1024)
	minRatio = 0.8
	address  = "127.0.0.1:2222"
)

type transferResults struct {
	mu    sync.Mutex
	done  map[string]chan sftpx.Transfer
	items map[string]sftpx.Transfer
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "benchmark SFTP:", err)
		os.Exit(1)
	}
}

func run() error {
	sizeMB, err := benchmarkSize(os.Args[1:])
	if err != nil {
		return err
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "openterminal-bench-sftp-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	source := filepath.Join(work, "bench.bin")
	fmt.Printf("Gerando %d MB aleatórios…\n", sizeMB)
	if err := randomFile(source, int64(sizeMB)*megabyte); err != nil {
		return err
	}
	expected, err := fileSHA256(source)
	if err != nil {
		return err
	}

	keySource := filepath.Join(root, "tests", "fixtures", "keys", "generated", "ed25519")
	exeUp, exeDown, err := benchmarkWindowsSFTP(work, keySource, source, expected)
	if err != nil {
		return err
	}
	goUp, goDown, err := benchmarkOpenTerminal(work, keySource, source, expected)
	if err != nil {
		return err
	}

	fmt.Printf("\n%d MB — MB/s (sftp.exe | OpenTerminal Go | razão)\n", sizeMB)
	failed := false
	for _, row := range []struct {
		label         string
		exe, openterminal time.Duration
	}{{"Envio", exeUp, goUp}, {"Download", exeDown, goDown}} {
		exeRate := float64(sizeMB) / row.exe.Seconds()
		goRate := float64(sizeMB) / row.openterminal.Seconds()
		ratio := goRate / exeRate
		failed = failed || ratio < minRatio
		fmt.Printf("%-9s %8.1f | %11.1f | %3.0f%%\n", row.label, exeRate, goRate, ratio*100)
	}
	if failed {
		return fmt.Errorf("OpenTerminal ficou abaixo de %.0f%% do sftp.exe", minRatio*100)
	}
	fmt.Println("\nOK: hashes idênticos e SC-006 atendido.")
	return nil
}

func benchmarkSize(args []string) (int, error) {
	if len(args) > 1 {
		return 0, fmt.Errorf("uso: go run ./scripts/bench-sftp [MB]")
	}
	if len(args) == 0 {
		return 1024, nil
	}
	size, err := strconv.Atoi(args[0])
	if err != nil || size < 1 {
		return 0, fmt.Errorf("tamanho inválido em MB")
	}
	return size, nil
}

func randomFile(path string, size int64) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	buffer := make([]byte, 8*megabyte)
	for written := int64(0); written < size; {
		chunk := int64(len(buffer))
		if remaining := size - written; remaining < chunk {
			chunk = remaining
		}
		if _, err := rand.Read(buffer[:chunk]); err != nil {
			_ = file.Close()
			return err
		}
		if _, err := file.Write(buffer[:chunk]); err != nil {
			_ = file.Close()
			return err
		}
		written += chunk
	}
	return file.Close()
}

func fileSHA256(path string) ([sha256.Size]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return [sha256.Size]byte{}, err
	}
	var sum [sha256.Size]byte
	copy(sum[:], hash.Sum(nil))
	return sum, nil
}

func benchmarkWindowsSFTP(work, keySource, source string, expected [sha256.Size]byte) (time.Duration, time.Duration, error) {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	sftpExe := filepath.Join(root, "System32", "OpenSSH", "sftp.exe")
	if _, err := os.Stat(sftpExe); err != nil {
		return 0, 0, fmt.Errorf("sftp.exe não encontrado: %w", err)
	}
	key := filepath.Join(work, "id_ed25519")
	if err := copyFile(keySource, key); err != nil {
		return 0, 0, err
	}
	username := os.Getenv("USERNAME")
	if username == "" {
		return 0, 0, fmt.Errorf("USERNAME não definido")
	}
	if output, err := exec.Command("icacls", key, "/inheritance:r", "/grant:r", username+":R").CombinedOutput(); err != nil {
		return 0, 0, fmt.Errorf("restringir chave do sftp.exe: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	download := filepath.Join(work, "exe-down.bin")
	putBatch := filepath.Join(work, "put.txt")
	getBatch := filepath.Join(work, "get.txt")
	if err := os.WriteFile(putBatch, []byte(fmt.Sprintf("put %s bench-exe.bin\n", quoteSFTPPath(source))), 0o600); err != nil {
		return 0, 0, err
	}
	if err := os.WriteFile(getBatch, []byte(fmt.Sprintf("get bench-exe.bin %s\nrm bench-exe.bin\n", quoteSFTPPath(download))), 0o600); err != nil {
		return 0, 0, err
	}
	run := func(batch string) error {
		command := exec.Command(sftpExe, "-q", "-i", key, "-P", "2222",
			"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=NUL", "-b", batch, "keyuser@127.0.0.1")
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
		return command.Run()
	}
	start := time.Now()
	if err := run(putBatch); err != nil {
		return 0, 0, fmt.Errorf("envio com sftp.exe: %w", err)
	}
	upload := time.Since(start)
	start = time.Now()
	if err := run(getBatch); err != nil {
		return 0, 0, fmt.Errorf("download com sftp.exe: %w", err)
	}
	downloadTime := time.Since(start)
	got, err := fileSHA256(download)
	if err != nil {
		return 0, 0, err
	}
	if got != expected {
		return 0, 0, fmt.Errorf("sftp.exe: hash diferente")
	}
	return upload, downloadTime, nil
}

func benchmarkOpenTerminal(work, keySource, source string, expected [sha256.Size]byte) (time.Duration, time.Duration, error) {
	privateKey, err := os.ReadFile(keySource)
	if err != nil {
		return 0, 0, err
	}
	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return 0, 0, err
	}
	client, err := ssh.Dial("tcp", address, &ssh.ClientConfig{
		User: "keyuser", Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 15 * time.Second,
	})
	if err != nil {
		return 0, 0, fmt.Errorf("conectar ao sshd de teste: %w", err)
	}
	defer client.Close()
	endpoint, err := sftpx.NewEndpoint("benchmark", client)
	if err != nil {
		return 0, 0, err
	}
	defer endpoint.Close()
	home, err := endpoint.RealPath(".")
	if err != nil {
		return 0, 0, err
	}

	results := &transferResults{done: map[string]chan sftpx.Transfer{}, items: map[string]sftpx.Transfer{}}
	var manager *sftpx.TransferManager
	manager = sftpx.NewTransferManager(sftpx.TransferEvents{
		Transfer: results.record,
		Conflict: func(conflict sftpx.ConflictEvent) {
			_ = manager.ResolveConflict(conflict.TransferID, sftpx.ConflictReplace)
		},
	}, sftpx.TransferManagerOptions{})
	wait := func(id string) error {
		transfer := results.wait(id)
		if transfer.State != sftpx.TransferCompleted {
			return fmt.Errorf("transferência %s: %s (%v)", transfer.ID, transfer.State, transfer.Reason)
		}
		return nil
	}

	start := time.Now()
	ids, err := manager.Upload(endpoint, []string{source}, home)
	if err != nil || len(ids) != 1 {
		return 0, 0, fmt.Errorf("iniciar envio: ids=%v: %w", ids, err)
	}
	if err := wait(ids[0]); err != nil {
		return 0, 0, err
	}
	upload := time.Since(start)

	downloadDir := filepath.Join(work, "go-down")
	if err := os.Mkdir(downloadDir, 0o700); err != nil {
		return 0, 0, err
	}
	remote := strings.TrimRight(home, "/") + "/" + filepath.Base(source)
	start = time.Now()
	ids, err = manager.Download(endpoint, []string{remote}, downloadDir)
	if err != nil || len(ids) != 1 {
		return 0, 0, fmt.Errorf("iniciar download: ids=%v: %w", ids, err)
	}
	if err := wait(ids[0]); err != nil {
		return 0, 0, err
	}
	download := time.Since(start)
	got, err := fileSHA256(filepath.Join(downloadDir, filepath.Base(source)))
	if err != nil {
		return 0, 0, err
	}
	if got != expected {
		return 0, 0, fmt.Errorf("OpenTerminal: hash diferente")
	}
	if err := endpoint.Remove(remote, false); err != nil {
		return 0, 0, err
	}
	return upload, download, nil
}

func (r *transferResults) record(transfer sftpx.Transfer) {
	r.mu.Lock()
	r.items[transfer.ID] = transfer
	channel := r.done[transfer.ID]
	if channel == nil {
		channel = make(chan sftpx.Transfer, 1)
		r.done[transfer.ID] = channel
	}
	final := transfer.State == sftpx.TransferCompleted || transfer.State == sftpx.TransferCancelled ||
		transfer.State == sftpx.TransferFailed || transfer.State == sftpx.TransferSkipped
	r.mu.Unlock()
	if final {
		select {
		case channel <- transfer:
		default:
		}
	}
}

func (r *transferResults) wait(id string) sftpx.Transfer {
	r.mu.Lock()
	if transfer, ok := r.items[id]; ok && (transfer.State == sftpx.TransferCompleted ||
		transfer.State == sftpx.TransferCancelled || transfer.State == sftpx.TransferFailed || transfer.State == sftpx.TransferSkipped) {
		r.mu.Unlock()
		return transfer
	}
	channel := r.done[id]
	if channel == nil {
		channel = make(chan sftpx.Transfer, 1)
		r.done[id] = channel
	}
	r.mu.Unlock()
	return <-channel
}

func copyFile(source, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

func quoteSFTPPath(path string) string {
	return `"` + strings.ReplaceAll(path, `"`, `""`) + `"`
}
