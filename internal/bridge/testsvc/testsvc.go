//go:build e2e

// Package testsvc existe só no build de testes (tag e2e): controla a janela e expõe contadores
// internos para os testes E2E (research R18). O build de release não contém este código.
package testsvc

import (
	"context"
	"os"
	"runtime"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Sessions é o que o TestService precisa do gerenciador de sessões.
type Sessions interface {
	Count() int
	// SizeOf devolve o último tamanho de PTY enviado ao servidor.
	SizeOf(sessionID string) (cols, rows int, ok bool)
}

type Size struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

type TestService struct {
	ctx      func() context.Context
	sessions func() Sessions
}

func New(ctx func() context.Context, sessions func() Sessions) *TestService {
	return &TestService{ctx: ctx, sessions: sessions}
}

func (t *TestService) Maximise() { wruntime.WindowMaximise(t.ctx()) }

func (t *TestService) Restore() {
	wruntime.WindowUnmaximise(t.ctx())
	wruntime.WindowUnminimise(t.ctx())
}

func (t *TestService) SetSize(width, height int) {
	wruntime.WindowUnmaximise(t.ctx())
	wruntime.WindowSetSize(t.ctx(), width, height)
}

// SetDialogPath troca o arquivo devolvido pelos diálogos de abrir/salvar
// (OPENTERMINAL_TEST_DIALOG_PATH) sem reiniciar o app: importar e exportar no mesmo teste.
func (t *TestService) SetDialogPath(path string) error {
	return os.Setenv("OPENTERMINAL_TEST_DIALOG_PATH", path)
}

func (t *TestService) Goroutines() int { return runtime.NumGoroutine() }

func (t *TestService) SessionCount() int {
	if s := t.sessions(); s != nil {
		return s.Count()
	}
	return 0
}

func (t *TestService) ServerSizeOf(sessionID string) *Size {
	s := t.sessions()
	if s == nil {
		return nil
	}
	cols, rows, ok := s.SizeOf(sessionID)
	if !ok {
		return nil
	}
	return &Size{Cols: cols, Rows: rows}
}
