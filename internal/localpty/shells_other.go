//go:build !windows

package localpty

import (
	"os"
	"os/exec"
	"path/filepath"

	"openterminal/internal/apperr"
)

// Fora do Windows não há registro para localizar o Git for Windows.
func gitRegistryDirs() []string { return nil }

var unixShells = []struct {
	id    Shell
	label string
}{
	{Bash, "Bash"},
	{Zsh, "Zsh"},
	{Fish, "Fish"},
	{Sh, "sh"},
}

func platformShellID(id Shell) bool {
	for _, candidate := range unixShells {
		if candidate.id == id {
			return true
		}
	}
	return false
}

func platformShells() []LocalShell {
	shells := []LocalShell{}
	for _, candidate := range unixShells {
		if shell, err := resolvePlatformShell(candidate.id); err == nil {
			shells = append(shells, shell)
		}
	}
	return shells
}

func resolvePlatformShell(id Shell) (LocalShell, error) {
	for _, candidate := range unixShells {
		if candidate.id != id {
			continue
		}
		path, err := exec.LookPath(string(id))
		if err != nil {
			break
		}
		// -l: shell de login, carrega ~/.profile como o terminal do sistema.
		return LocalShell{ID: string(id), Label: candidate.label, Kind: KindUnix, path: absolutePath(path), args: []string{"-l"}}, nil
	}
	return LocalShell{}, apperr.New(apperr.InvalidInput)
}

// DefaultShell é o shell de login do usuário ($SHELL) quando suportado; senão o primeiro
// disponível, na ordem de unixShells.
func DefaultShell() Shell {
	if login := Shell(filepath.Base(os.Getenv("SHELL"))); login != "" {
		if _, err := resolvePlatformShell(login); err == nil {
			return login
		}
	}
	if shells := platformShells(); len(shells) > 0 {
		return Shell(shells[0].ID)
	}
	return Bash
}

// ptyEnv completa o ambiente herdado: aberto pelo menu, o app não recebe TERM.
func ptyEnv() []string {
	return []string{"TERM=xterm-256color", "COLORTERM=truecolor"}
}
