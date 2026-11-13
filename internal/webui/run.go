//go:build linux

package webui

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"openterminal/internal/bridge"
)

// Quanto tempo o backend espera por uma página antes de desistir, e quanto tempo segura sem
// nenhuma página depois que o navegador fechou (um F5 reconecta em menos que isso).
const (
	startGrace = 30 * time.Second
	idleGrace  = 5 * time.Second
)

// Run abre o app no Chrome/Chromium. Devolve false sem fazer nada quando não há navegador
// compatível (o main.go cai para a janela WebKitGTK) ou quando OPENTERMINAL_WEBKIT=1.
func Run(assets fs.FS, dataDir string) (bool, error) {
	if os.Getenv("OPENTERMINAL_WEBKIT") == "1" {
		return false, nil
	}
	browser, ok := FindBrowser(dataDir)
	if !ok {
		return false, nil
	}
	lock, err := lockInstance(dataDir)
	if err != nil {
		// Backend anterior ainda vivo (a janela acabou de fechar e ele espera alguns segundos
		// por um F5): em vez de desistir, abre uma janela nova apontando para ele.
		if reopenErr := reopen(browser, dataDir); reopenErr == nil {
			return true, nil
		}
		return true, err
	}
	defer lock.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var server *Server
	bridge.SetShell(bridge.Shell{
		Emit:           func(_ context.Context, name string, payload any) { emit(server, name, payload) },
		Quit:           func(context.Context) { cancel() },
		Minimise:       func(context.Context) {},
		ToggleMaximise: func(context.Context) {},
		OpenURL:        func(_ context.Context, url string) { _ = exec.Command("xdg-open", url).Start() },
		// A página escreve na área de transferência (navigator.clipboard); o X11 não tem uma
		// ferramenta garantida para isso no backend.
		ClipboardSetText: func(_ context.Context, text string) error { emit(server, "app:clipboard", text); return nil },
		OpenFile:         openFile,
		OpenFiles:        openFiles,
		OpenDirectory:    openDirectory,
		SaveFile:         saveFile,
	})
	services, err := bridge.NewServices(dataDir)
	if err != nil {
		return true, err
	}
	server, err = NewServer(assets, services.Bound(), bridge.FormatError)
	if err != nil {
		return true, err
	}
	go func() {
		if err := server.Serve(); err != nil {
			fmt.Fprintln(os.Stderr, "servidor local:", err)
			cancel()
		}
	}()
	services.Startup(ctx)
	defer func() {
		services.Shutdown(ctx)
		_ = server.Close()
	}()

	urlFile := filepath.Join(dataDir, "instance.url")
	_ = os.WriteFile(urlFile, []byte(server.URL()), 0o600)
	defer os.Remove(urlFile)

	cmd, err := browser.Command(server.URL())
	if err != nil {
		return true, err
	}
	if err := cmd.Start(); err != nil {
		return true, fmt.Errorf("abrir %s: %w", browser.Name, err)
	}
	go maximiseWhenMapped()
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	defer func() {
		select {
		case <-exited:
		default:
			_ = cmd.Process.Signal(syscall.SIGTERM)
		}
	}()
	waitDone(ctx, server, exited)
	return true, nil
}

// waitDone volta quando o app pede para sair ou quando o navegador fechou e não sobrou página.
// Não basta esperar o processo: se aquele perfil já estava aberto, o Chrome repassa a janela
// para o processo existente e sai na hora.
func waitDone(ctx context.Context, server *Server, exited <-chan struct{}) {
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	start := time.Now()
	var idleSince time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		if server.Connected() > 0 {
			idleSince = time.Time{}
			continue
		}
		if idleSince.IsZero() {
			idleSince = time.Now()
		}
		browserGone := false
		select {
		case <-exited:
			browserGone = true
		default:
		}
		everConnected := server.everConnected()
		switch {
		case !everConnected && time.Since(start) > startGrace:
			fmt.Fprintln(os.Stderr, "o navegador não conectou; encerrando")
			return
		case everConnected && browserGone && time.Since(idleSince) > idleGrace:
			return
		case everConnected && !browserGone && time.Since(idleSince) > 10*idleGrace:
			// Navegador vivo sem página nossa: a janela fechou e o Chrome ficou em segundo plano.
			return
		}
	}
}

// lockInstance: um backend por pasta de dados. A segunda execução avisa e sai, sem tocar no cofre.
func lockInstance(dataDir string) (*os.File, error) {
	f, err := os.OpenFile(filepath.Join(dataDir, "instance.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errors.New("o OpenTerminal já está aberto")
		}
		return nil, err
	}
	return f, nil
}

// emit ignora eventos de antes do servidor existir (só a construção dos serviços roda antes).
func emit(server *Server, name string, payload any) {
	if server != nil {
		server.Emit(name, payload)
	}
}

// maximiseWhenMapped maximiza a janela do app assim que ela aparece. No Cinnamon (Muffin) o
// Chromium liga a barra de título do sistema depois de mapear a janela e o quadro fica errado:
// a barra vai para baixo do conteúdo (faixa vazia) e não sobra onde arrastar. Maximizar refaz o
// quadro, e restaurar depois mantém tudo certo. Usa o wmctrl se existir; sem ele, nada muda.
// --start-maximized não serve: o Chromium prefere a posição salva no perfil.
func maximiseWhenMapped() {
	wmctrl, err := exec.LookPath("wmctrl")
	if err != nil {
		return
	}
	for i := 0; i < 40; i++ {
		time.Sleep(250 * time.Millisecond)
		out, err := exec.Command(wmctrl, "-lx").Output()
		if err != nil {
			return
		}
		for _, line := range strings.Split(string(out), "\n") {
			f := strings.Fields(line)
			if len(f) >= 3 && strings.HasSuffix(f[2], ".OpenTerminal") {
				_ = exec.Command(wmctrl, "-ir", f[0], "-b", "add,maximized_vert,maximized_horz").Run()
				return
			}
		}
	}
}

// reopen abre uma janela para o backend que já está rodando, pelo endereço que ele deixou.
func reopen(browser Browser, dataDir string) error {
	url, err := os.ReadFile(filepath.Join(dataDir, "instance.url"))
	if err != nil || len(url) == 0 {
		return errors.New("sem endereço da instância aberta")
	}
	cmd, err := browser.Command(strings.TrimSpace(string(url)))
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go maximiseWhenMapped()
	go func() { _ = cmd.Wait() }()
	time.Sleep(3 * time.Second) // dá tempo de o Chromium repassar a janela e de maximizar
	return nil
}
