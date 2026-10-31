package localpty

import (
	"encoding/binary"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"unicode/utf16"
)

func TestDetectShellsWithInjectedReaders(t *testing.T) {
	root := `C:\Windows`
	programFiles := `C:\Program Files`
	pwsh := filepath.Join(programFiles, `PowerShell\7\pwsh.exe`)
	git := filepath.Join(`D:\Apps\Git`, `bin\bash.exe`)
	wsl := filepath.Join(root, `System32\wsl.exe`)
	existing := map[string]bool{
		filepath.Join(root, `System32\WindowsPowerShell\v1.0\powershell.exe`): true,
		filepath.Join(root, `System32\cmd.exe`):                               true,
		pwsh:                                                                  true, git: true, wsl: true,
	}
	readers := shellReaders{
		getenv: func(key string) string {
			if key == "SystemRoot" {
				return root
			}
			if key == "ProgramFiles" {
				return programFiles
			}
			return ""
		},
		lookPath: func(name string) (string, error) {
			if name == "wsl.exe" {
				t.Fatal("wsl.exe do PATH consultado apesar do executável do sistema")
			}
			return "", errors.New("ausente")
		},
		exists: func(path string) bool { return existing[path] },
		glob: func(pattern string) ([]string, error) {
			return []string{pwsh}, nil
		},
		registryDirs: func() []string { return []string{`D:\Apps\Git`} },
		output: func(path string, args ...string) ([]byte, error) {
			if path != wsl {
				t.Fatalf("executável WSL = %q", path)
			}
			switch {
			case reflect.DeepEqual(args, []string{"-l", "-q"}):
				return utf16LE("Ubuntu\r\ndocker-desktop\r\nDOCKER-DESKTOP-DATA\r\nLegacyUnknown\r\n"), nil
			case reflect.DeepEqual(args, []string{"-l", "-v"}):
				return utf16LE("  NAME STATE VERSION\r\n* Ubuntu Running 2\r\n  LegacyUnknown Stopped 0\r\n"), nil
			default:
				t.Fatalf("argumentos WSL = %v", args)
				return nil, nil
			}
		},
	}

	got := detectShells(readers)
	want := []struct {
		id, label string
		kind      ShellKind
		args      []string
	}{
		{"powershell", "Windows PowerShell", KindPowerShell, nil},
		{"pwsh", "PowerShell 7", KindPwsh, nil},
		{"cmd", "Prompt de Comando", KindCmd, nil},
		{"gitbash", "Git Bash", KindGitBash, []string{"--login", "-i"}},
		{"wsl:Ubuntu", "Ubuntu", KindWSL, []string{"-d", "Ubuntu"}},
	}
	if len(got) != len(want) {
		t.Fatalf("shells = %+v", got)
	}
	for i := range want {
		if got[i].ID != want[i].id || got[i].Label != want[i].label || got[i].Kind != want[i].kind || !reflect.DeepEqual(got[i].args, want[i].args) {
			t.Errorf("shell %d = %+v, esperado %+v", i, got[i], want[i])
		}
	}
}

func TestDetectShellsUsesPATHAndOmitsMissing(t *testing.T) {
	pwsh := `D:\PowerShell\pwsh.exe`
	readers := shellReaders{
		getenv: func(string) string { return "" },
		lookPath: func(name string) (string, error) {
			if name == "pwsh.exe" {
				return pwsh, nil
			}
			return "", errors.New("ausente")
		},
		exists:       func(path string) bool { return path == pwsh },
		glob:         filepath.Glob,
		registryDirs: func() []string { return nil },
		output:       func(string, ...string) ([]byte, error) { return nil, errors.New("não executado") },
	}
	got := detectShells(readers)
	if len(got) != 1 || got[0].ID != "pwsh" {
		t.Fatalf("shells = %+v", got)
	}
}

func TestParseWSLDistrosUTF16AndValidIDs(t *testing.T) {
	got := parseWSLDistros(utf16LE("\ufeffUbuntu\r\nUbuntu\r\n openSUSE Tumbleweed \r\ndocker-desktop\r\nDocker-Desktop-Data\r\n"))
	if !reflect.DeepEqual(got, []string{"Ubuntu", "openSUSE Tumbleweed"}) {
		t.Fatalf("distros = %v", got)
	}
	for _, id := range []string{"powershell", "pwsh", "cmd", "gitbash", "wsl:Ubuntu", "wsl:openSUSE Tumbleweed"} {
		if !ValidShellID(id) {
			t.Errorf("id válido recusado: %q", id)
		}
	}
	for _, id := range []string{"", "bash", "wsl:", "wsl: Ubuntu", "wsl:Ubuntu\nDebian"} {
		if ValidShellID(id) {
			t.Errorf("id inválido aceito: %q", id)
		}
	}
}

func TestResolveCmdDoesNotEnumerateWSL(t *testing.T) {
	root := `C:\Windows`
	cmd := filepath.Join(root, `System32\cmd.exe`)
	readers := shellReaders{
		getenv: func(key string) string {
			if key == "SystemRoot" {
				return root
			}
			return ""
		},
		lookPath: func(string) (string, error) { return "", errors.New("não deveria procurar no PATH") },
		exists:   func(path string) bool { return path == cmd },
		glob:     func(string) ([]string, error) { return nil, errors.New("não deveria enumerar") },
		registryDirs: func() []string {
			t.Fatal("não deveria consultar o registro")
			return nil
		},
		output: func(string, ...string) ([]byte, error) {
			t.Fatal("não deveria executar wsl")
			return nil, nil
		},
	}

	got, err := resolveShellWithReaders(Cmd, readers)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "cmd" || got.path != cmd {
		t.Fatalf("shell = %+v", got)
	}
}

func utf16LE(text string) []byte {
	words := utf16.Encode([]rune(text))
	output := make([]byte, len(words)*2)
	for i, word := range words {
		binary.LittleEndian.PutUint16(output[i*2:], word)
	}
	return output
}
