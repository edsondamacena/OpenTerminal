//go:build integration

package bridge

import (
	"context"
	"net"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"openterminal/internal/apperr"
	"openterminal/internal/domain"
	"openterminal/internal/model"
)

func trustIntegrationHostKey(t *testing.T, services *Services) {
	t.Helper()
	var keyType, fingerprint string
	client, err := ssh.Dial("tcp", "127.0.0.1:2222", &ssh.ClientConfig{
		User: "pwuser", Auth: []ssh.AuthMethod{ssh.Password("pw-test-123")}, Timeout: 10 * time.Second,
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			keyType, fingerprint = key.Type(), ssh.FingerprintSHA256(key)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = client.Close()
	if err := services.store.Mutate(func(content *model.Content) error {
		domain.AcceptHostKey(content, domain.EndpointOf("127.0.0.1", 2222), keyType, fingerprint, domain.Now())
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSftpServiceIntegrationLifecycleAndDialogs(t *testing.T) {
	services := testServices(t)
	if err := services.vaultService.Create("senha-teste-1", "senha-teste-1"); err != nil {
		t.Fatal(err)
	}
	credential, err := services.credentialService.CreatePassword("Senha", "pw-test-123")
	if err != nil {
		t.Fatal(err)
	}
	host, err := services.hostService.Create(domain.HostInput{
		Label: "SFTP", Address: "127.0.0.1", Port: 2222, Username: "pwuser",
		AuthMethod: model.AuthPassword, CredentialID: &credential.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	trustIntegrationHostKey(t, services)

	opened, err := services.sftpService.Open(host.ID)
	if err != nil {
		t.Fatal(err)
	}
	work := path.Join(opened.HomePath, "bridge-"+time.Now().Format("150405.000000000"))
	if err := services.sftpService.Mkdir(opened.SessionID, work); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = services.sftpService.Remove(opened.SessionID, work, true) })

	local := filepath.Join(t.TempDir(), "origem.txt")
	if err := os.WriteFile(local, []byte("olá via ponte"), 0o600); err != nil {
		t.Fatal(err)
	}
	services.sftpService.pickFiles = func(context.Context) ([]string, error) { return []string{local}, nil }
	upload, err := services.sftpService.Upload(opened.SessionID, work)
	if err != nil || len(upload.TransferIDs) != 1 {
		t.Fatalf("upload: %+v %v", upload, err)
	}
	waitBridgeSFTP(t, func() bool {
		entries, err := services.sftpService.List(opened.SessionID, work)
		return err == nil && len(entries) == 1 && entries[0].Name == "origem.txt"
	})

	destination := t.TempDir()
	services.sftpService.pickDir = func(context.Context) (string, error) { return destination, nil }
	download, err := services.sftpService.Download(opened.SessionID, []string{path.Join(work, "origem.txt")})
	if err != nil || len(download.TransferIDs) != 1 {
		t.Fatalf("download: %+v %v", download, err)
	}
	waitBridgeSFTP(t, func() bool {
		data, err := os.ReadFile(filepath.Join(destination, "origem.txt"))
		return err == nil && string(data) == "olá via ponte"
	})

	if err := services.sftpService.Rename(opened.SessionID, path.Join(work, "origem.txt"), path.Join(work, "renomeado.txt")); err != nil {
		t.Fatal(err)
	}
	if err := services.sftpService.Remove(opened.SessionID, path.Join(work, "renomeado.txt"), false); err != nil {
		t.Fatal(err)
	}
	if err := services.sessionService.Close(opened.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := services.sftpService.List(opened.SessionID, work); apperr.CodeOf(err) != apperr.SessionClosed {
		t.Fatalf("sessão fechada: %v", err)
	}
}

func waitBridgeSFTP(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("tempo esgotado esperando operação SFTP pela ponte")
}
