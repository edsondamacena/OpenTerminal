//go:build windows

package update

import "os/exec"

// Open roda o instalador NSIS (o Windows pede a confirmação do UAC).
func Open(path string) error { return exec.Command(path).Start() }
