//go:build linux && !android

package devicekey

import (
	"os"
	"testing"

	"github.com/godbus/dbus/v5"
)

// Só com OT_KEYRING_CLEAN=1: apaga todos os itens de desbloqueio do OpenTerminal do chaveiro.
func TestCleanAllUnlockItems(t *testing.T) {
	if os.Getenv("OT_KEYRING_CLEAN") != "1" {
		t.Skip()
	}
	conn, svc, session, err := open()
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll(conn, session)
	var unlocked, locked []dbus.ObjectPath
	q := map[string]string{"application": "openterminal", "kind": "vault-unlock"}
	if err := svc.Call("org.freedesktop.Secret.Service.SearchItems", 0, q).Store(&unlocked, &locked); err != nil {
		t.Fatal(err)
	}
	for _, p := range append(unlocked, locked...) {
		var prompt dbus.ObjectPath
		_ = conn.Object(busName, p).Call("org.freedesktop.Secret.Item.Delete", 0).Store(&prompt)
	}
	t.Logf("apagados: %d", len(unlocked)+len(locked))
}
