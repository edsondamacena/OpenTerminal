package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	goruntime "runtime"
	"sync/atomic"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"openterminal/internal/appwin"
	"openterminal/internal/bridge"
	"openterminal/internal/webui"
)

//go:embed all:frontend/dist
var assets embed.FS

// Preenchidos só pelo build de testes (main_e2e.go, tag e2e).
var (
	extraWebViewArgs []string
	extraBindings    func(*bridge.Services) []any
)

func main() {
	if err := run(); err != nil {
		// Sem arquivo de log (FR-030): o erro vai só para stderr, sem dados do usuário.
		fmt.Fprintln(os.Stderr, "OpenTerminal não pôde iniciar:", err)
		os.Exit(1)
	}
}

func run() error {
	dataDir, err := appwin.DataDir()
	if err != nil {
		return err
	}
	// No Linux a interface roda no Chrome/Chromium do sistema: o WebKitGTK do Wails desenha o
	// terminal bem mais devagar. Sem navegador compatível, segue a janela do Wails.
	dist, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		return err
	}
	if !generatingBindings {
		if handled, err := webui.Run(dist, dataDir); handled {
			return err
		}
	}
	if err := appwin.ClearWebViewAutofillData(dataDir); err != nil {
		return fmt.Errorf("limpar dados antigos de preenchimento automático: %w", err)
	}
	appwin.ConfigureWebView(extraWebViewArgs)

	services, err := bridge.NewServices(dataDir)
	if err != nil {
		return err
	}
	bind := services.Bound()
	if extraBindings != nil {
		bind = append(bind, extraBindings(services)...)
	}

	// Lido pela goroutine da segunda instância; escrito no OnStartup.
	var appCtx atomic.Pointer[context.Context]
	return wails.Run(&options.App{
		Title:              "OpenTerminal",
		Width:              1280,
		Height:             800,
		MinWidth:           800,
		MinHeight:          500,
		Frameless:          true,
		BackgroundColour:   backgroundColour(),
		AssetServer:        &assetserver.Options{Assets: assets},
		LogLevel:           logger.ERROR,
		LogLevelProduction: logger.ERROR,
		ErrorFormatter:     bridge.FormatError,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: instanceID(dataDir),
			// Segunda execução não abre outra janela: traz a existente para frente (FR-111).
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				ctx := appCtx.Load()
				if ctx == nil {
					return
				}
				runtime.WindowUnminimise(*ctx)
				runtime.Show(*ctx)
			},
		},
		OnStartup: func(ctx context.Context) {
			appCtx.Store(&ctx)
			services.Startup(ctx)
		},
		OnBeforeClose: services.BeforeClose,
		OnShutdown:    services.Shutdown,
		Bind:          bind,
		Windows: &windows.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			BackdropType:         windows.Mica,
			Theme:                windows.SystemDefault,
			WebviewUserDataPath:  appwin.WebViewDir(dataDir),
		},
		Linux: &linux.Options{
			// Aceleração por GPU sempre ligada: com OnDemand o WebKitGTK só usa a GPU quando a
			// página pede composição e alterna de modo no meio do uso.
			WebviewGpuPolicy: linux.WebviewGpuPolicyAlways,
		},
	})
}

// backgroundColour: no Windows a janela é transparente para o Mica aparecer; no Linux não há
// Mica e uma janela transparente só custa composição, então o fundo é opaco.
func backgroundColour() *options.RGBA {
	if goruntime.GOOS == "windows" {
		return &options.RGBA{R: 0, G: 0, B: 0, A: 0}
	}
	return &options.RGBA{R: 18, G: 18, B: 18, A: 255}
}

// instanceID: uma instância por diretório de dados. Os testes E2E usam diretórios temporários
// e não esbarram no app aberto pelo usuário.
func instanceID(dataDir string) string {
	sum := sha256.Sum256([]byte(dataDir))
	return "openterminal-" + hex.EncodeToString(sum[:8])
}
