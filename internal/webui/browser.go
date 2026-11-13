package webui

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
)

// Browser é um Chrome/Chromium instalado: o comando e a pasta de perfil que ele consegue usar.
type Browser struct {
	Name    string
	command []string
	profile string
}

var nativeBrowsers = []string{"chromium", "chromium-browser", "google-chrome-stable", "google-chrome", "brave-browser", "microsoft-edge-stable"}

var flatpakBrowsers = []string{"org.chromium.Chromium", "com.google.Chrome", "com.brave.Browser", "io.github.ungoogled_software.ungoogled_chromium"}

// FindBrowser procura, nesta ordem: OPENTERMINAL_BROWSER, binários no PATH e apps flatpak.
// Perfil próprio do OpenTerminal, separado do navegador do usuário; no flatpak ele fica em
// ~/.var/app/<id>, a única pasta da home que o sandbox sempre enxerga.
func FindBrowser(dataDir string) (Browser, bool) {
	own := filepath.Join(dataDir, "browser")
	if path := os.Getenv("OPENTERMINAL_BROWSER"); path != "" {
		return Browser{Name: path, command: []string{path}, profile: own}, true
	}
	for _, name := range nativeBrowsers {
		if path, err := exec.LookPath(name); err == nil {
			return Browser{Name: name, command: []string{path}, profile: own}, true
		}
	}
	flatpak, err := exec.LookPath("flatpak")
	if err != nil {
		return Browser{}, false
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Browser{}, false
	}
	for _, id := range flatpakBrowsers {
		if exec.Command(flatpak, "info", id).Run() == nil {
			return Browser{
				Name:    id,
				command: []string{flatpak, "run", id},
				profile: filepath.Join(home, ".var", "app", id, "openterminal-profile"),
			}, true
		}
	}
	return Browser{}, false
}

// Command monta a janela de app: sem barra de endereço, WM_CLASS OpenTerminal (ícone certo no
// dock) e perfil sem sincronização, extensões nem gerenciador de senhas.
func (b Browser) Command(url string) (*exec.Cmd, error) {
	if err := prepareProfile(b.profile); err != nil {
		return nil, err
	}
	args := append(append([]string{}, b.command[1:]...),
		"--app="+url,
		"--class=OpenTerminal",
		"--user-data-dir="+b.profile,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-extensions",
		"--disable-sync",
		"--password-store=basic",
		"--disable-features=Translate",
		"--window-size=1280,800",
	)
	return exec.Command(b.command[0], args...), nil
}

// prepareProfile cria o perfil já com o gerenciador de senhas e o preenchimento automático
// desligados: a senha mestra não pode ir parar no cofre do navegador.
func prepareProfile(dir string) error {
	prefs := filepath.Join(dir, "Default", "Preferences")
	if _, err := os.Stat(prefs); !errors.Is(err, os.ErrNotExist) {
		if err != nil {
			return err
		}
		return useSystemFrame(prefs)
	}
	if err := os.MkdirAll(filepath.Dir(prefs), 0o700); err != nil {
		return err
	}
	data, _ := json.Marshal(map[string]any{
		"credentials_enable_service": false,
		"profile":                    map[string]any{"password_manager_enabled": false},
		"autofill":                   map[string]any{"profile_enabled": false, "credit_card_enabled": false},
		"translate":                  map[string]any{"enabled": false},
		"browser":                    map[string]any{"custom_chrome_frame": false},
	})
	return os.WriteFile(prefs, data, 0o600)
}

// useSystemFrame liga a barra de título do sistema no perfil já existente. Com o quadro próprio do
// Chrome, no Cinnamon a janela ficava sem barra para arrastar e com uma faixa de sombra vazia
// em volta do conteúdo. O navegador ainda não está rodando aqui, então pode gravar.
func useSystemFrame(prefs string) error {
	raw, err := os.ReadFile(prefs)
	if err != nil {
		return err
	}
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return nil // perfil ilegível: o Chrome refaz; não vale travar a abertura por isso
	}
	browser, _ := data["browser"].(map[string]any)
	if browser == nil {
		browser = map[string]any{}
	}
	if v, ok := browser["custom_chrome_frame"].(bool); ok && !v {
		return nil
	}
	browser["custom_chrome_frame"] = false
	data["browser"] = browser
	out, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(prefs, out, 0o600)
}
