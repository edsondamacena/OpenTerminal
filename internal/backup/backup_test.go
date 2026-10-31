package backup

import (
	"bytes"
	"encoding/json"
	"testing"

	"openterminal/internal/model"
	"openterminal/internal/vault"
)

func TestRoundTripAndTamper(t *testing.T) {
	p := Payload{Hosts: []model.Host{{ID: "h", Label: "OLT"}}, Credentials: []model.Credential{{ID: "c", Kind: model.CredentialPassword, Password: "segredo-de-teste"}}}
	data, err := Encode(p, "senha-do-backup", vault.TestKDF)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("segredo-de-teste")) || bytes.Contains(data, []byte("OLT")) {
		t.Fatal("conteúdo em claro no arquivo")
	}
	got, err := Decode(data, "senha-do-backup")
	if err != nil || got.Hosts[0].Label != "OLT" || got.Credentials[0].Password != "segredo-de-teste" {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := Decode(data, "outra-senha"); err != ErrBadPassword {
		t.Fatalf("senha errada: %v", err)
	}
	var f map[string]any
	_ = json.Unmarshal(data, &f)
	f["kdf"].(map[string]any)["time"] = 2
	forged, _ := json.Marshal(f)
	if _, err := Decode(forged, "senha-do-backup"); err != ErrBadPassword {
		t.Fatalf("parâmetro alterado passou: %v", err)
	}
	f["kdf"].(map[string]any)["memoryKiB"] = 100 * 1024 * 1024
	forged, _ = json.Marshal(f)
	if _, err := Decode(forged, "senha-do-backup"); err != ErrNotBackup {
		t.Fatalf("KDF absurdo aceito: %v", err)
	}
	if _, err := Encode(p, "curta", vault.TestKDF); err != ErrWeak {
		t.Fatal("senha curta aceita")
	}
	if _, err := Decode([]byte(`{"format":"outro"}`), "x"); err != ErrNotBackup {
		t.Fatal("arquivo estranho aceito")
	}
}
