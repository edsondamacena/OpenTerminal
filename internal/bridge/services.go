// Package bridge expõe ao Wails os serviços de contracts/bridge.md. Só as structs devolvidas
// por Bound são ligadas; métodos exportados delas viram funções chamáveis pela interface.
// Services em si não é ligado: seus métodos são o ciclo de vida chamado pelo main.go.
package bridge

import (
	"context"
	"sync"
	"sync/atomic"

	"openterminal/internal/apperr"
	"openterminal/internal/appwin"
	"openterminal/internal/forwarding"
	"openterminal/internal/session"
	"openterminal/internal/sftpx"
	"openterminal/internal/sshx"
	"openterminal/internal/vault"
)

// allServicesWired passa a true quando os 8 serviços do contrato estiverem ligados (T094).
const allServicesWired = true

// Services agrupa os serviços ligados ao Wails e o estado compartilhado entre eles.
type Services struct {
	dataDir string

	ctxMu sync.RWMutex
	ctx   context.Context

	store       *vault.Store
	autoLock    *vault.AutoLock
	sessions    *session.Manager
	forwards    *forwarding.Manager
	transfers   *sftpx.TransferManager
	forwardDial func(context.Context, sshx.Options, session.Interactor) (forwarding.Client, error)

	vaultService      *VaultService
	hostService       *HostService
	credentialService *CredentialService
	sessionService    *SessionService
	sftpService       *SftpService
	forwardService    *ForwardService
	profileService    *ProfileService
	appService        *AppService
	syncService       *SyncService
	closeGuard        *appwin.CloseGuard
	// closing: o Close do cofre ao sair também avisa "bloqueado", mas não é decisão do usuário.
	closing atomic.Bool
}

func NewServices(dataDir string) (*Services, error) {
	return newServices(dataDir, vault.DefaultKDF)
}

func newServices(dataDir string, kdf func([]byte) vault.KDFParams) (*Services, error) {
	store, err := vault.Open(dataDir, kdf)
	if err != nil {
		return nil, err
	}
	s := &Services{dataDir: dataDir, store: store}
	s.autoLock = vault.NewAutoLock(store)
	s.sessions = session.NewManager(s.emit)
	s.vaultService = newVaultService(store, s.autoLock)
	s.hostService = newHostService(store, s.Context)
	s.credentialService = newCredentialService(store, s.Context)
	s.sessionService = newSessionService(store, s.sessions)
	s.transfers = sftpx.NewTransferManager(sftpx.TransferEvents{
		Transfer: func(transfer sftpx.Transfer) { s.emit("sftp:transfer", transfer) },
		Conflict: func(conflict sftpx.ConflictEvent) { s.emit("sftp:conflict", conflict) },
	}, sftpx.TransferManagerOptions{})
	s.sftpService = newSftpService(s.sessions, s.sessionService, s.transfers, s.Context)
	s.sessionService.sftpDial = func(sessionID, hostID string) session.Dialer {
		return s.sftpService.dial(sessionID, hostID, nil)
	}
	s.sessionService.transferCount = s.transfers.ActiveCount
	s.forwardDial = func(ctx context.Context, options sshx.Options, interactor session.Interactor) (forwarding.Client, error) {
		return sshx.Dial(ctx, options, interactor)
	}
	s.forwards = forwarding.NewManager(s.connectForward, func(event forwarding.StatusEvent) { s.emit("forwards:status", event) })
	s.forwardService = newForwardService(store, s.forwards)
	s.profileService = newProfileService(store, s.sessions, s.Context, s.emit)
	preferences := appwin.NewPreferenceStore(store)
	appearance := appwin.NewAppearanceMonitor(appwin.RegistryAppearanceReader{}, preferences.Get,
		func(value appwin.Appearance) { s.emit("app:appearance", value) })
	s.closeGuard = appwin.NewCloseGuard(
		func() appwin.ActiveSummary {
			return appwin.ActiveSummary{Sessions: s.sessions.ActiveCount(), Transfers: s.transfers.ActiveCount(), Forwards: s.forwards.ActiveCount()}
		},
		func(summary appwin.ActiveSummary) { s.emit("app:close-requested", summary) },
		func() {
			if ctx := s.Context(); ctx != nil {
				shell.Quit(ctx)
			}
		},
	)
	s.appService = newAppService(preferences, appearance, s.closeGuard, s.Context)
	store.OnStatus(func(status vault.Status) { s.emit("vault:state", status) })
	s.syncService = newSyncService(store, kdf, s.Context, s.emit)
	s.vaultService.onUnlock = func(password string) {
		s.rememberUnlock()
		s.syncService.unlocked(password)
	}
	s.vaultService.onErase = s.forgetUnlock
	store.OnStatus(func(status vault.Status) {
		if status.State == vault.StateLocked && !s.closing.Load() {
			s.forgetUnlock()
		}
	})
	s.vaultService.onPasswordChange = s.syncService.passwordChanged
	return s, nil
}

func (s *Services) connectForward(hostID string) (forwarding.Client, error) {
	options, err := s.sessionService.optionsForHost(hostID)
	if err != nil {
		return nil, err
	}
	ctx := s.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	_, interactor, done := s.sessions.BeginInteraction(ctx)
	defer done()
	return s.forwardDial(ctx, options, interactor)
}

// Bound devolve as structs que o main.go passa para options.App.Bind.
func (s *Services) Bound() []any {
	return []any{s.vaultService, s.hostService, s.credentialService, s.sessionService, s.sftpService, s.forwardService, s.profileService, s.appService, s.syncService}
}

// Context é o contexto do Wails, disponível depois de Startup.
func (s *Services) Context() context.Context {
	s.ctxMu.RLock()
	defer s.ctxMu.RUnlock()
	return s.ctx
}

func (s *Services) emit(name string, payload any) {
	if ctx := s.Context(); ctx != nil {
		shell.Emit(ctx, name, payload)
	}
}

func (s *Services) Startup(ctx context.Context) {
	s.ctxMu.Lock()
	s.ctx = ctx
	s.ctxMu.Unlock()
	s.appService.appearance.Start(ctx)
	s.autoUnlock()
}

// BeforeClose devolve true para impedir o fechamento (confirmação de FR-028, T038).
func (s *Services) BeforeClose(ctx context.Context) bool {
	return s.closeGuard.Intercept()
}

func (s *Services) Shutdown(ctx context.Context) {
	s.closing.Store(true)
	s.forwards.CloseAll()
	s.sessions.CloseAll()
	_ = s.store.Close()
}

// SessionStats é o que os testes E2E leem do gerenciador de sessões.
type SessionStats interface {
	Count() int
	SizeOf(sessionID string) (cols, rows int, ok bool)
}

// TestSessions só é usado pelo build de testes (main_e2e.go).
func (s *Services) TestSessions() SessionStats {
	return s.sessions
}

func clean(err error) error {
	return apperr.Sanitize(err)
}

// FormatError mantém o formato textual esperado pelo adaptador web, mas garante que até erros
// residuais do dispatcher atravessem somente no envelope seguro do app.
func FormatError(err error) any {
	safe := apperr.Sanitize(err)
	if safe == nil {
		return nil
	}
	return safe.Error()
}
