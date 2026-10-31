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

	cmd, err := browser.Command(server.URL())
	if err != nil {
		return true, err
	}
	if err := cmd.Start(); err != nil {
		return true, fmt.Errorf("abrir %s: %w", browser.Name, err)
	}
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
