//go:build windows

package localpty

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

func gitRegistryDirs() []string {
	type location struct {
		root registry.Key
		path string
	}
	locations := []location{
		{registry.CURRENT_USER, `SOFTWARE\GitForWindows`},
		{registry.LOCAL_MACHINE, `SOFTWARE\GitForWindows`},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\GitForWindows`},
	}
	var dirs []string
	for _, location := range locations {
		key, err := registry.OpenKey(location.root, location.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		value, _, err := key.GetStringValue("InstallPath")
		_ = key.Close()
		if err == nil && strings.TrimSpace(value) != "" {
			dirs = append(dirs, value)
		}
	}
	return dirs
}

func platformShells() []LocalShell { return detectShells(systemShellReaders()) }

func resolvePlatformShell(id Shell) (LocalShell, error) {
	return resolveShellWithReaders(id, systemShellReaders())
}

// DefaultShell é o shell da nova aba quando o usuário não escolheu outro.
func DefaultShell() Shell { return PowerShell }

func ptyEnv() []string { return nil }

func platformShellID(Shell) bool { return false }
