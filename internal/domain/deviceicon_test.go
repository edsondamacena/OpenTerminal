package domain

import (
	"testing"

	"openterminal/internal/model"
)

func TestHostDeviceIcon(t *testing.T) {
	c := &model.Content{}
	in := HostInput{Label: "OLT", Address: "10.0.0.1", Port: 23, AuthMethod: model.AuthAsk,
		Protocol: model.ProtocolTelnet, DeviceIcon: "olt"}
	h, err := CreateHost(c, in, "now")
	if err != nil || h.DeviceIcon != "olt" {
		t.Fatalf("create: %v %q", err, h.DeviceIcon)
	}
	in.DeviceIcon = ""
	if h, err = UpdateHost(c, h.ID, in, "now"); err != nil || h.DeviceIcon != "" {
		t.Fatalf("padrão: %v %q", err, h.DeviceIcon)
	}
	in.DeviceIcon = "../x"
	if _, err = CreateHost(c, in, "now"); err == nil {
		t.Fatal("ícone desconhecido aceito")
	}
}
