package domain

import (
	"net"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"

	"openterminal/internal/model"
)

type HostKeyKind string

const (
	HostKeyNew     HostKeyKind = "new"
	HostKeyMatch   HostKeyKind = "match"
	HostKeyChanged HostKeyKind = "changed"
)

type HostKeyCheck struct {
	Kind     HostKeyKind
	Previous *model.KnownHost
}

// EndpointOf usa o mesmo formato da 001 ("host:porta", IPv6 entre colchetes), para que as host
// keys migradas continuem valendo.
func EndpointOf(address string, port int) string {
	a := strings.ToLower(NormalizeAddress(address))
	p := strconv.Itoa(port)
	if ip := net.ParseIP(a); ip != nil && strings.Contains(a, ":") {
		return "[" + a + "]:" + p
	}
	return a + ":" + p
}

// Fingerprint no formato do OpenSSH: "SHA256:" + base64 sem padding.
func Fingerprint(pub ssh.PublicKey) string {
	return ssh.FingerprintSHA256(pub)
}

func CheckHostKey(c *model.Content, endpoint, keyType, fingerprint string) HostKeyCheck {
	for _, k := range c.KnownHosts {
		if k.Endpoint != endpoint {
			continue
		}
		if k.KeyType == keyType && k.Fingerprint == fingerprint {
			return HostKeyCheck{Kind: HostKeyMatch}
		}
		prev := k
		return HostKeyCheck{Kind: HostKeyChanged, Previous: &prev}
	}
	return HostKeyCheck{Kind: HostKeyNew}
}

// AcceptHostKey grava a chave aceita, substituindo a anterior do mesmo endpoint.
func AcceptHostKey(c *model.Content, endpoint, keyType, fingerprint, now string) {
	out := c.KnownHosts[:0]
	for _, k := range c.KnownHosts {
		if k.Endpoint != endpoint {
			out = append(out, k)
		}
	}
	c.KnownHosts = append(out, model.KnownHost{ID: NewID(), Endpoint: endpoint, KeyType: keyType,
		Fingerprint: fingerprint, AcceptedAt: now})
}
