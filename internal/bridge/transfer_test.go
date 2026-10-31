package bridge

import (
	"testing"

	"openterminal/internal/backup"
	"openterminal/internal/domain"
	"openterminal/internal/model"
)

func TestMergeBackup(t *testing.T) {
	src := model.EmptyContent()
	now := "2026-10-27T00:00:00Z"
	root, _ := domain.CreateGroup(src, "Provedor", nil)
	child, _ := domain.CreateGroup(src, "OLTs", &root.ID)
	cred, _ := domain.CreatePassword(src, "noc", "segredo", now)
	olt, err := domain.CreateHost(src, domain.HostInput{Label: "OLT", Address: "10.0.0.1", Port: 23, Username: "noc",
		AuthMethod: model.AuthPassword, CredentialID: &cred.ID, GroupID: &child.ID, Protocol: model.ProtocolTelnet}, now)
	if err != nil {
		t.Fatal(err)
	}
	bng, _ := domain.CreateHost(src, domain.HostInput{Label: "BNG", Address: "10.0.0.2", Port: 22, Username: "root",
		AuthMethod: model.AuthPassword, CredentialID: &cred.ID, GroupID: &root.ID}, now)
	domain.AcceptHostKey(src, "10.0.0.2:22", "ssh-ed25519", "SHA256:x", now)
	if _, err := domain.CreateForwardRule(src, domain.ForwardRuleInput{HostID: bng.ID, BindAddress: "127.0.0.1", BindPort: 18080, DestHost: "10.0.0.9", DestPort: 80}); err != nil {
		t.Fatal(err)
	}
	p := backup.Payload{Groups: src.Groups, Hosts: src.Hosts, Credentials: src.Credentials, KnownHosts: src.KnownHosts, ForwardRules: src.ForwardRules}

	dst := model.EmptyContent()
	r := mergeBackup(dst, p, now)
	if r != (BackupImportResult{Hosts: 2, Groups: 2, Credentials: 1, KnownHosts: 1, ForwardRules: 1}) {
		t.Fatalf("1ª: %+v", r)
	}
	var got model.Host
	for _, h := range dst.Hosts {
		if h.Label == "OLT" {
			got = h
		}
	}
	if got.ID == olt.ID || !got.IsTelnet() || got.CredentialID == nil || *got.CredentialID == cred.ID {
		t.Fatalf("ids não foram trocados ou campos perdidos: %+v", got)
	}
	g, _ := domain.FindHost(dst, got.ID)
	var grp model.Group
	for _, x := range dst.Groups {
		if x.ID == *g.GroupID {
			grp = x
		}
	}
	if grp.Name != "OLTs" || grp.ParentID == nil {
		t.Fatalf("hierarquia: %+v", grp)
	}
	r = mergeBackup(dst, p, now)
	if r != (BackupImportResult{Duplicates: 2}) || len(dst.Hosts) != 2 || len(dst.Credentials) != 1 || len(dst.Groups) != 2 {
		t.Fatalf("2ª duplicou: %+v", r)
	}
}
