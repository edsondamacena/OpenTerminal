//go:build linux && !android

package devicekey

import (
	"errors"

	"github.com/godbus/dbus/v5"
)

const (
	busName    = "org.freedesktop.secrets"
	collection = dbus.ObjectPath("/org/freedesktop/secrets/aliases/default")
)

// SecretService usa o chaveiro da sessão (gnome-keyring, KWallet). Chaveiro bloqueado: não
// pede a senha do chaveiro; o app só cai na senha mestra.
type SecretService struct{}

func Default() Keeper { return SecretService{} }

type secret struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string
}

func attrs(name string) map[string]string {
	return map[string]string{"application": "openterminal", "kind": "vault-unlock", "name": name}
}

func open() (*dbus.Conn, dbus.BusObject, dbus.ObjectPath, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, nil, "", err
	}
	svc := conn.Object(busName, "/org/freedesktop/secrets")
	var out dbus.Variant
	var session dbus.ObjectPath
	if err := svc.Call("org.freedesktop.Secret.Service.OpenSession", 0, "plain", dbus.MakeVariant("")).Store(&out, &session); err != nil {
		conn.Close()
		return nil, nil, "", err
	}
	return conn, svc, session, nil
}

func closeAll(conn *dbus.Conn, session dbus.ObjectPath) {
	_ = conn.Object(busName, session).Call("org.freedesktop.Secret.Session.Close", 0).Err
	conn.Close()
}

func (SecretService) Get(name string) ([]byte, bool) {
	conn, svc, session, err := open()
	if err != nil {
		return nil, false
	}
	defer closeAll(conn, session)
	var unlocked, locked []dbus.ObjectPath
	if err := svc.Call("org.freedesktop.Secret.Service.SearchItems", 0, attrs(name)).Store(&unlocked, &locked); err != nil || len(unlocked) == 0 {
		return nil, false
	}
	var secrets map[dbus.ObjectPath]secret
	if err := svc.Call("org.freedesktop.Secret.Service.GetSecrets", 0, unlocked[:1], session).Store(&secrets); err != nil {
		return nil, false
	}
	s, ok := secrets[unlocked[0]]
	if !ok || len(s.Value) == 0 {
		return nil, false
	}
	return s.Value, true
}

func (SecretService) Set(name string, value []byte) error {
	conn, _, session, err := open()
	if err != nil {
		return err
	}
	defer closeAll(conn, session)
	props := map[string]dbus.Variant{
		"org.freedesktop.Secret.Item.Label":      dbus.MakeVariant("OpenTerminal — desbloqueio automático do cofre"),
		"org.freedesktop.Secret.Item.Attributes": dbus.MakeVariant(attrs(name)),
	}
	var item, prompt dbus.ObjectPath
	err = conn.Object(busName, collection).Call("org.freedesktop.Secret.Collection.CreateItem", 0,
		props, secret{Session: session, Value: value, ContentType: "application/octet-stream"}, true).Store(&item, &prompt)
	if err != nil {
		return err
	}
	if prompt != "/" {
		return errors.New("chaveiro do sistema bloqueado")
	}
	return nil
}

func (SecretService) Delete(name string) error {
	conn, svc, session, err := open()
	if err != nil {
		return err
	}
	defer closeAll(conn, session)
	var unlocked, locked []dbus.ObjectPath
	if err := svc.Call("org.freedesktop.Secret.Service.SearchItems", 0, attrs(name)).Store(&unlocked, &locked); err != nil {
		return err
	}
	for _, p := range append(unlocked, locked...) {
		var prompt dbus.ObjectPath
		_ = conn.Object(busName, p).Call("org.freedesktop.Secret.Item.Delete", 0).Store(&prompt)
	}
	return nil
}
