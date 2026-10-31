//go:build linux

package tabby

import (
	"github.com/godbus/dbus/v5"
)

// KeyringLookup lê do Secret Service (gnome-keyring, KWallet) no formato do keytar, que o Tabby
// usa: atributos service e account. Chaveiro bloqueado ou ausente: nil, e o import segue sem
// essas senhas.
func KeyringLookup() (Lookup, func()) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, func() {}
	}
	service := conn.Object("org.freedesktop.secrets", "/org/freedesktop/secrets")
	var output dbus.Variant
	var session dbus.ObjectPath
	if err := service.Call("org.freedesktop.Secret.Service.OpenSession", 0, "plain", dbus.MakeVariant("")).Store(&output, &session); err != nil {
		conn.Close()
		return nil, func() {}
	}
	closeAll := func() {
		_ = conn.Object("org.freedesktop.secrets", session).Call("org.freedesktop.Secret.Session.Close", 0).Err
		conn.Close()
	}
	lookup := func(svc, account string) (string, bool) {
		var unlocked, locked []dbus.ObjectPath
		attrs := map[string]string{"service": svc, "account": account}
		if err := service.Call("org.freedesktop.Secret.Service.SearchItems", 0, attrs).Store(&unlocked, &locked); err != nil || len(unlocked) == 0 {
			return "", false
		}
		// (session, parameters, value, content_type)
		var secrets map[dbus.ObjectPath]struct {
			Session     dbus.ObjectPath
			Parameters  []byte
			Value       []byte
			ContentType string
		}
		if err := service.Call("org.freedesktop.Secret.Service.GetSecrets", 0, unlocked[:1], session).Store(&secrets); err != nil {
			return "", false
		}
		s, ok := secrets[unlocked[0]]
		if !ok || len(s.Value) == 0 {
			return "", false
		}
		return string(s.Value), true
	}
	return lookup, closeAll
}
