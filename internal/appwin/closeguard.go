package appwin

import "sync"

type ActiveSummary struct {
	Sessions  int `json:"sessions"`
	Transfers int `json:"transfers"`
	Forwards  int `json:"forwards"`
}

// CloseGuard impede o fechamento enquanto há trabalho ativo até a confirmação da interface.
type CloseGuard struct {
	mu        sync.Mutex
	confirmed bool
	summary   func() ActiveSummary
	request   func(ActiveSummary)
	close     func()
}

func NewCloseGuard(summary func() ActiveSummary, request func(ActiveSummary), close func()) *CloseGuard {
	return &CloseGuard{summary: summary, request: request, close: close}
}

// Intercept informa ao Wails se o fechamento deve ser impedido.
func (g *CloseGuard) Intercept() bool {
	g.mu.Lock()
	confirmed := g.confirmed
	g.mu.Unlock()
	if confirmed {
		return false
	}
	summary := g.summary()
	if summary.Sessions+summary.Transfers+summary.Forwards == 0 {
		return false
	}
	g.request(summary)
	return true
}

func (g *CloseGuard) Confirm(confirmed bool) {
	if !confirmed {
		return
	}
	g.mu.Lock()
	if g.confirmed {
		g.mu.Unlock()
		return
	}
	g.confirmed = true
	g.mu.Unlock()
	g.close()
}
