//go:build linux && !android

package devicekey

import (
	"bytes"
	"os"
	"testing"
)

// Só com OT_KEYRING_PROBE=1: grava, lê e apaga um item de teste no chaveiro real da sessão.
func TestSecretServiceRoundTrip(t *testing.T) {
	if os.Getenv("OT_KEYRING_PROBE") != "1" {
		t.Skip()
	}
	k := Default()
	const name = "probe-test"
	if err := k.Set(name, []byte("abc")); err != nil {
		t.Fatal(err)
	}
	got, ok := k.Get(name)
	if !ok || !bytes.Equal(got, []byte("abc")) {
		t.Fatalf("leu %q %v", got, ok)
	}
	if err := k.Delete(name); err != nil {
		t.Fatal(err)
	}
	if _, ok := k.Get(name); ok {
		t.Fatal("não apagou")
	}
}
