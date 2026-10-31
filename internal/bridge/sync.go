package bridge

import (
	"context"
	"errors"
	"sync"
	"time"

	"openterminal/internal/apperr"
	"openterminal/internal/cloudsync"
	"openterminal/internal/gdrive"
	"openterminal/internal/vault"
)

const (
	googlePref   = "cloudsync.google"
	googleSecret = "google-refresh-token"
)

// googleLink fica nas preferências; o token vai cifrado com a chave de dados do cofre.
type googleLink struct {
	Token []byte `json:"token"`
	Email string `json:"email"`
}

// SyncStatus é o que a tela de sincronização mostra.
type SyncStatus struct {
	cloudsync.Status
	Configured bool   `json:"configured"`
	Email      string `json:"email"`
	LoggingIn  bool   `json:"loggingIn"`
}

// SyncService liga o cofre ao Google Drive (cloudsync + gdrive).
type SyncService struct {
	store   *vault.Store
	engine  *cloudsync.Engine
	context func() context.Context
	emit    func(string, any)

	mu     sync.Mutex
	email  string
	cancel context.CancelFunc // login em andamento
}

func newSyncService(store *vault.Store, kdf func([]byte) vault.KDFParams, ctx func() context.Context, emit func(string, any)) *SyncService {
	s := &SyncService{store: store, context: ctx, emit: emit}
	s.engine = cloudsync.NewEngine(store, kdf, func(cloudsync.Status) { s.emit("sync:status", s.status()) })
	s.engine.KeyReady = s.keepKey
	store.OnChange(s.engine.Changed)
	store.OnStatus(func(st vault.Status) {
		if st.State != vault.StateUnlocked {
			s.engine.Locked()
		}
	})
	return s
}

func (s *SyncService) status() SyncStatus {
	_, err := gdrive.LoadClient()
	s.mu.Lock()
	defer s.mu.Unlock()
	return SyncStatus{Status: s.engine.Status(), Configured: err == nil, Email: s.email, LoggingIn: s.cancel != nil}
}

// unlocked roda logo depois de destravar (ou criar) o cofre: retoma o sync se houver conta ligada.
func (s *SyncService) unlocked(password string) {
	var link googleLink
	if ok, err := s.store.Preference(googlePref, &link); err != nil || !ok || len(link.Token) == 0 {
		s.engine.SetRemote(nil) // cofre apagado e recriado: a conta antiga não vale mais
		return
	}
	if s.connectRemote(link) {
		s.engine.Unlocked(password)
	}
}

// connectRemote liga o Drive com o token guardado (cifrado com a chave de dados).
func (s *SyncService) connectRemote(link googleLink) bool {
	s.mu.Lock()
	s.email = link.Email
	s.mu.Unlock()
	client, err := gdrive.LoadClient()
	if err != nil {
		return false
	}
	token, err := s.store.OpenSecret(googleSecret, link.Token)
	if err != nil {
		return false
	}
	s.engine.SetRemote(gdrive.NewDrive(client, string(token)))
	vault.Zero(token)
	return true
}

func (s *SyncService) passwordChanged(old, next string) { s.engine.PasswordChanged(old, next) }

func (s *SyncService) GetStatus() (SyncStatus, error) { return s.status(), nil }

// Connect abre o login Google no navegador e, com a conta ligada, faz a primeira sincronização.
// A senha mestra deriva a chave da nuvem: é a mesma em todos os aparelhos.
func (s *SyncService) Connect(password string) (SyncStatus, error) {
	if err := s.store.CheckPassword(password); err != nil {
		return SyncStatus{}, err
	}
	client, err := gdrive.LoadClient()
	if err != nil {
		return s.failed(err), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return SyncStatus{}, apperr.New(apperr.InUse)
	}
	s.cancel = cancel
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.cancel = nil
		s.mu.Unlock()
	}()
	s.emit("sync:status", s.status())

	grant, err := gdrive.Login(ctx, client, func(u string) {
		if c := s.context(); c != nil {
			shell.OpenURL(c, u)
		}
	})
	if errors.Is(ctx.Err(), context.Canceled) {
		return s.status(), nil
	}
	if err != nil {
		return s.failed(err), nil
	}
	sealed, err := s.store.SealSecret(googleSecret, []byte(grant.RefreshToken))
	if err != nil {
		return SyncStatus{}, err
	}
	if err := s.store.SetPreference(googlePref, googleLink{Token: sealed, Email: grant.Email}); err != nil {
		return SyncStatus{}, err
	}
	s.mu.Lock()
	s.email = grant.Email
	s.mu.Unlock()
	syncCtx, done := context.WithTimeout(context.Background(), 2*time.Minute)
	defer done()
	_ = s.engine.Connect(syncCtx, gdrive.NewDrive(client, grant.RefreshToken), password)
	return s.status(), nil
}

// CancelConnect desiste do login em andamento (a aba do Google pode ficar aberta).
func (s *SyncService) CancelConnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

// Disconnect desliga a conta neste aparelho e revoga o acesso no Google. O arquivo na nuvem
// fica (os outros aparelhos continuam usando).
func (s *SyncService) Disconnect() (SyncStatus, error) {
	var link googleLink
	if ok, _ := s.store.Preference(googlePref, &link); ok && len(link.Token) > 0 {
		if token, err := s.store.OpenSecret(googleSecret, link.Token); err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_ = gdrive.Revoke(ctx, string(token))
			cancel()
			vault.Zero(token)
		}
	}
	if err := s.store.SetPreference(googlePref, googleLink{}); err != nil {
		return SyncStatus{}, err
	}
	s.engine.SetRemote(nil)
	s.mu.Lock()
	s.email = ""
	s.mu.Unlock()
	return s.status(), nil
}

func (s *SyncService) SyncNow() (SyncStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	_ = s.engine.SyncNow(ctx)
	return s.status(), nil
}

// failed mostra o erro na tela de sync sem sair pelo canal de erro (que esconde o texto).
func (s *SyncService) failed(err error) SyncStatus {
	st := s.status()
	st.State, st.Error = "error", err.Error()
	return st
}
