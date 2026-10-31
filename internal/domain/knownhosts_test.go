package domain

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"openterminal/internal/model"
)

func TestEndpointOf(t *testing.T) {
	cases := map[string]string{
		EndpointOf("SRV.Example.com", 22): "srv.example.com:22",
		EndpointOf("[2001:DB8::1]", 2222): "[2001:db8::1]:2222",
		EndpointOf("::1", 22):             "[::1]:22",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	}
}

func TestFingerprintMatches001(t *testing.T) {
	pubKey, _, _ := ed25519.GenerateKey(rand.Reader)
	pub, _ := ssh.NewPublicKey(pubKey)
	sum := sha256.Sum256(pub.Marshal())
	want := "SHA256:" + strings.TrimRight(base64.StdEncoding.EncodeToString(sum[:]), "=")
	if got := Fingerprint(pub); got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestHostKeyLifecycle(t *testing.T) {
	c := model.EmptyContent()
	ep := EndpointOf("h", 22)
	if CheckHostKey(c, ep, "ssh-ed25519", "SHA256:aaa").Kind != HostKeyNew {
		t.Fatal("deveria ser nova")
	}
	AcceptHostKey(c, ep, "ssh-ed25519", "SHA256:aaa", t0)
	if CheckHostKey(c, ep, "ssh-ed25519", "SHA256:aaa").Kind != HostKeyMatch {
		t.Fatal("deveria conferir")
	}
	ch := CheckHostKey(c, ep, "ssh-ed25519", "SHA256:bbb")
	if ch.Kind != HostKeyChanged || ch.Previous == nil || ch.Previous.Fingerprint != "SHA256:aaa" {
		t.Fatalf("%+v", ch)
	}
	if CheckHostKey(c, ep, "ssh-rsa", "SHA256:aaa").Kind != HostKeyChanged {
		t.Fatal("tipo diferente é divergência")
	}
}

func TestAcceptReplaces(t *testing.T) {
	c := model.EmptyContent()
	ep := EndpointOf("h", 22)
	AcceptHostKey(c, ep, "ssh-ed25519", "SHA256:aaa", t0)
	AcceptHostKey(c, ep, "ssh-ed25519", "SHA256:bbb", t1)
	if len(c.KnownHosts) != 1 || c.KnownHosts[0].Fingerprint != "SHA256:bbb" || c.KnownHosts[0].AcceptedAt != t1 || c.KnownHosts[0].ID == "" {
		t.Fatalf("%+v", c.KnownHosts)
	}
}

func TestPortsAreDistinct(t *testing.T) {
	c := model.EmptyContent()
	AcceptHostKey(c, EndpointOf("h", 22), "ssh-ed25519", "SHA256:aaa", t0)
	if CheckHostKey(c, EndpointOf("h", 2222), "ssh-ed25519", "SHA256:aaa").Kind != HostKeyNew {
		t.Fatal("porta diferente deveria ser nova")
	}
}
