package localpty

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"openterminal/internal/apperr"
)

type Shell string

const (
	PowerShell Shell = "powershell"
	Pwsh       Shell = "pwsh"
	Cmd        Shell = "cmd"
	GitBash    Shell = "gitbash"

	// Shells de Linux/macOS, detectados por shells_other.go.
	Bash Shell = "bash"
	Zsh  Shell = "zsh"
	Fish Shell = "fish"
	Sh   Shell = "sh"
)

type ShellKind string

const (
	KindPowerShell ShellKind = "powershell"
	KindPwsh       ShellKind = "pwsh"
	KindCmd        ShellKind = "cmd"
	KindGitBash    ShellKind = "gitbash"
	KindWSL        ShellKind = "wsl"
	KindUnix       ShellKind = "unix"
)

// LocalShell descreve um shell instalado sem expor o caminho do executável para a interface.
// path e args ficam no backend e são usados ao abrir a sessão.
type LocalShell struct {
	ID    string    `json:"id"`
	Label string    `json:"label"`
	Kind  ShellKind `json:"kind"`
	path  string
	args  []string
}

type shellReaders struct {
	getenv       func(string) string
	lookPath     func(string) (string, error)
	exists       func(string) bool
	glob         func(string) ([]string, error)
	registryDirs func() []string
	output       func(string, ...string) ([]byte, error)
}

func systemShellReaders() shellReaders {
	return shellReaders{
		getenv:   os.Getenv,
		lookPath: exec.LookPath,
		exists: func(path string) bool {
			info, err := os.Stat(path)
			return err == nil && !info.IsDir()
		},
		glob:         filepath.Glob,
		registryDirs: gitRegistryDirs,
		output: func(name string, args ...string) ([]byte, error) {
			return exec.Command(name, args...).Output()
		},
	}
}

// ListShells devolve apenas shells presentes na máquina, em ordem estável.
func ListShells() []LocalShell {
	return platformShells()
}

// ValidShellID valida ids persistidos, inclusive o id dinâmico de uma distribuição WSL.
func ValidShellID(id string) bool {
	switch Shell(id) {
	case PowerShell, Pwsh, Cmd, GitBash:
		return true
	}
	if platformShellID(Shell(id)) {
		return true
	}
	if !strings.HasPrefix(id, "wsl:") {
		return false
	}
	distro := strings.TrimPrefix(id, "wsl:")
	return distro != "" && distro == strings.TrimSpace(distro) && utf8.ValidString(distro) &&
		utf8.RuneCountInString(distro) <= 128 && !strings.ContainsAny(distro, "\x00\r\n")
}

func detectShells(r shellReaders) []LocalShell {
	root := r.getenv("SystemRoot")
	if root == "" || !filepath.IsAbs(root) {
		root = `C:\Windows`
	}
	programFiles := r.getenv("ProgramFiles")

	// Nunca nil: a interface lê o tamanho da lista e null quebraria a inicialização.
	shells := []LocalShell{}
	add := func(id Shell, label string, kind ShellKind, path string, args ...string) {
		if path == "" || !r.exists(path) {
			return
		}
		for _, shell := range shells {
			if shell.ID == string(id) {
				return
			}
		}
		shells = append(shells, LocalShell{ID: string(id), Label: label, Kind: kind, path: path, args: args})
	}

	add(PowerShell, "Windows PowerShell", KindPowerShell,
		filepath.Join(root, `System32\WindowsPowerShell\v1.0\powershell.exe`))

	if path, err := r.lookPath("pwsh.exe"); err == nil {
		add(Pwsh, "PowerShell 7", KindPwsh, absolutePath(path))
	} else if programFiles != "" {
		matches, _ := r.glob(filepath.Join(programFiles, `PowerShell\*\pwsh.exe`))
		sort.Sort(sort.Reverse(sort.StringSlice(matches)))
		for _, match := range matches {
			if r.exists(match) {
				add(Pwsh, "PowerShell 7", KindPwsh, match)
				break
			}
		}
	}

	add(Cmd, "Prompt de Comando", KindCmd, filepath.Join(root, `System32\cmd.exe`))

	if programFiles != "" {
		add(GitBash, "Git Bash", KindGitBash, filepath.Join(programFiles, `Git\bin\bash.exe`), "--login", "-i")
	}
	if !containsShell(shells, string(GitBash)) {
		for _, dir := range r.registryDirs() {
			path := filepath.Join(dir, `bin\bash.exe`)
			if r.exists(path) {
				add(GitBash, "Git Bash", KindGitBash, path, "--login", "-i")
				break
			}
		}
	}

	wslPath := ""
	candidate := filepath.Join(root, `System32\wsl.exe`)
	if r.exists(candidate) {
		wslPath = candidate
	} else if path, err := r.lookPath("wsl.exe"); err == nil {
		wslPath = absolutePath(path)
	}
	if wslPath != "" {
		if output, err := r.output(wslPath, "-l", "-q"); err == nil {
			distros := parseWSLDistros(output)
			versions := map[string]string{}
			if verbose, verboseErr := r.output(wslPath, "-l", "-v"); verboseErr == nil {
				versions = parseWSLVersions(verbose)
			}
			sort.Slice(distros, func(i, j int) bool { return strings.ToLower(distros[i]) < strings.ToLower(distros[j]) })
			for _, distro := range distros {
				if version, known := versions[strings.ToLower(distro)]; known && version != "1" && version != "2" {
					continue
				}
				shells = append(shells, LocalShell{
					ID: "wsl:" + distro, Label: distro, Kind: KindWSL,
					path: wslPath, args: []string{"-d", distro},
				})
			}
		}
	}
	return shells
}

func containsShell(shells []LocalShell, id string) bool {
	for _, shell := range shells {
		if shell.ID == id {
			return true
		}
	}
	return false
}

func absolutePath(path string) string {
	absolute, err := filepath.Abs(path)
	if err == nil {
		return absolute
	}
	return path
}

func parseWSLDistros(output []byte) []string {
	text := decodeWindowsOutput(output)
	seen := make(map[string]bool)
	var distros []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		distro := strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		internal := strings.EqualFold(distro, "docker-desktop") || strings.EqualFold(distro, "docker-desktop-data")
		if distro == "" || internal || seen[distro] {
			continue
		}
		seen[distro] = true
		distros = append(distros, distro)
	}
	return distros
}

func parseWSLVersions(output []byte) map[string]string {
	versions := map[string]string{}
	text := decodeWindowsOutput(output)
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "\ufeff")))
		if len(fields) < 2 {
			continue
		}
		if fields[0] == "*" {
			fields = fields[1:]
		}
		if len(fields) < 2 {
			continue
		}
		version := fields[len(fields)-1]
		if version == "VERSION" {
			continue
		}
		versions[strings.ToLower(fields[0])] = version
	}
	return versions
}

func decodeWindowsOutput(output []byte) string {
	if len(output) < 2 {
		return string(output)
	}
	littleEndian := output[0] == 0xff && output[1] == 0xfe
	bigEndian := output[0] == 0xfe && output[1] == 0xff
	if !littleEndian && !bigEndian {
		for i := 1; i < len(output); i += 2 {
			if output[i] == 0 {
				littleEndian = true
				break
			}
		}
	}
	if !littleEndian && !bigEndian {
		return string(output)
	}
	if output[0] == 0xff || output[0] == 0xfe {
		output = output[2:]
	}
	words := make([]uint16, 0, len(output)/2)
	for i := 0; i+1 < len(output); i += 2 {
		if bigEndian {
			words = append(words, binary.BigEndian.Uint16(output[i:i+2]))
		} else {
			words = append(words, binary.LittleEndian.Uint16(output[i:i+2]))
		}
	}
	return string(utf16.Decode(words))
}

func resolveShell(id Shell) (LocalShell, error) {
	return resolvePlatformShell(id)
}

// resolveShellWithReaders procura somente o shell solicitado. Em particular, abrir cmd ou
// PowerShell não enumera distribuições WSL nem executa processos externos.
func resolveShellWithReaders(id Shell, r shellReaders) (LocalShell, error) {
	root := r.getenv("SystemRoot")
	if root == "" || !filepath.IsAbs(root) {
		root = `C:\Windows`
	}
	programFiles := r.getenv("ProgramFiles")
	found := func(label string, kind ShellKind, path string, args ...string) (LocalShell, error) {
		if path == "" || !r.exists(path) {
			return LocalShell{}, apperr.New(apperr.InvalidInput)
		}
		return LocalShell{ID: string(id), Label: label, Kind: kind, path: path, args: args}, nil
	}

	switch id {
	case PowerShell:
		return found("Windows PowerShell", KindPowerShell,
			filepath.Join(root, `System32\WindowsPowerShell\v1.0\powershell.exe`))
	case Cmd:
		return found("Prompt de Comando", KindCmd, filepath.Join(root, `System32\cmd.exe`))
	case Pwsh:
		if path, err := r.lookPath("pwsh.exe"); err == nil && r.exists(path) {
			return found("PowerShell 7", KindPwsh, absolutePath(path))
		}
		if programFiles != "" {
			matches, _ := r.glob(filepath.Join(programFiles, `PowerShell\*\pwsh.exe`))
			sort.Sort(sort.Reverse(sort.StringSlice(matches)))
			for _, match := range matches {
				if r.exists(match) {
					return found("PowerShell 7", KindPwsh, match)
				}
			}
		}
	case GitBash:
		if programFiles != "" {
			path := filepath.Join(programFiles, `Git\bin\bash.exe`)
			if r.exists(path) {
				return found("Git Bash", KindGitBash, path, "--login", "-i")
			}
		}
		for _, dir := range r.registryDirs() {
			path := filepath.Join(dir, `bin\bash.exe`)
			if r.exists(path) {
				return found("Git Bash", KindGitBash, path, "--login", "-i")
			}
		}
	default:
		if ValidShellID(string(id)) && strings.HasPrefix(string(id), "wsl:") {
			path := filepath.Join(root, `System32\wsl.exe`)
			if !r.exists(path) {
				var err error
				path, err = r.lookPath("wsl.exe")
				if err != nil {
					break
				}
				path = absolutePath(path)
			}
			distro := strings.TrimPrefix(string(id), "wsl:")
			return found(distro, KindWSL, path, "-d", distro)
		}
	}
	return LocalShell{}, apperr.New(apperr.InvalidInput)
}

// ShellPath devolve o caminho absoluto do executável detectado.
func ShellPath(shell Shell) (string, error) {
	resolved, err := resolveShell(shell)
	if err != nil {
		return "", err
	}
	return resolved.path, nil
}
