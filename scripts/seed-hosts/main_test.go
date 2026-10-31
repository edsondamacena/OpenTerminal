package main

import (
	"testing"

	"openterminal/internal/vault"
)

func TestRunSeedsGroupsAndHosts(t *testing.T) {
	dir := t.TempDir()
	const password = "senha-teste-1"
	if err := runWithKDF(dir, password, 11, vault.TestKDF); err != nil {
		t.Fatal(err)
	}
	store, err := vault.Open(dir, vault.TestKDF)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Unlock(password); err != nil {
		t.Fatal(err)
	}
	content, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(content.Groups) != 25 {
		t.Fatalf("grupos = %d, esperado 25", len(content.Groups))
	}
	if len(content.Hosts) != 11 {
		t.Fatalf("hosts = %d, esperado 11", len(content.Hosts))
	}
	var first, secondFound bool
	for _, host := range content.Hosts {
		if host.Label == "srv-00001" {
			first = host.GroupID == nil
		}
		if host.Label == "srv-00002" {
			secondFound = host.GroupID != nil
		}
	}
	if !first {
		t.Fatal("srv-00001 deveria existir na raiz")
	}
	if !secondFound {
		t.Fatal("srv-00002 deveria pertencer a um grupo")
	}
}
