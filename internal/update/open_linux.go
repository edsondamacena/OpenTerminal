//go:build linux && !android

package update

import "os/exec"

// Open entrega o .deb ao instalador de pacotes do sistema (ele pede a confirmação e a senha).
func Open(path string) error { return exec.Command("xdg-open", path).Start() }
