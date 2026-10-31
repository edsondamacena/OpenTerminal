package bridge

import (
	"context"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Shell é o que os serviços pedem à janela: eventos, diálogos, área de transferência e o ciclo
// de vida. O padrão é o runtime do Wails; no Linux o app roda no Chromium (internal/webui) e troca
// tudo por SetShell antes de NewServices.
type Shell struct {
	Emit             func(ctx context.Context, name string, payload any)
	Quit             func(ctx context.Context)
	Minimise         func(ctx context.Context)
	ToggleMaximise   func(ctx context.Context)
	OpenURL          func(ctx context.Context, url string)
	ClipboardSetText func(ctx context.Context, text string) error
	OpenFile         func(ctx context.Context, options wruntime.OpenDialogOptions) (string, error)
	OpenFiles        func(ctx context.Context, options wruntime.OpenDialogOptions) ([]string, error)
	OpenDirectory    func(ctx context.Context, options wruntime.OpenDialogOptions) (string, error)
	SaveFile         func(ctx context.Context, options wruntime.SaveDialogOptions) (string, error)
}

func wailsShell() Shell {
	return Shell{
		Emit:             func(ctx context.Context, name string, payload any) { wruntime.EventsEmit(ctx, name, payload) },
		Quit:             wruntime.Quit,
		Minimise:         wruntime.WindowMinimise,
		ToggleMaximise:   wruntime.WindowToggleMaximise,
		OpenURL:          wruntime.BrowserOpenURL,
		ClipboardSetText: wruntime.ClipboardSetText,
		OpenFile:         wruntime.OpenFileDialog,
		OpenFiles:        wruntime.OpenMultipleFilesDialog,
		OpenDirectory:    wruntime.OpenDirectoryDialog,
		SaveFile:         wruntime.SaveFileDialog,
	}
}

// shell é global porque há um único app por processo; só SetShell escreve, antes dos serviços.
var shell = wailsShell()

// SetShell troca o Shell. Chamar antes de NewServices.
func SetShell(s Shell) { shell = s }
