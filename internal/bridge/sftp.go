package bridge

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"openterminal/internal/apperr"
	"openterminal/internal/domain"
	"openterminal/internal/session"
	"openterminal/internal/sftpx"
	"openterminal/internal/sshx"
	"openterminal/internal/validate"
)

const maxRemotePathBytes = 4096

type OpenSFTPResult struct {
	SessionID string `json:"sessionId"`
	HomePath  string `json:"homePath"`
}

type TransferIDs struct {
	TransferIDs []string `json:"transferIds"`
}

type SftpService struct {
	sessions  *session.Manager
	terminal  *SessionService
	transfers *sftpx.TransferManager
	context   func() context.Context

	mu        sync.RWMutex
	endpoints map[string]*sftpx.Endpoint

	pickFiles func(context.Context) ([]string, error)
	pickDir   func(context.Context) (string, error)
	dialSSH   func(context.Context, sshx.Options, session.Interactor) (*sshx.Client, error)
}

func newSftpService(
	sessions *session.Manager,
	terminal *SessionService,
	transfers *sftpx.TransferManager,
	contextFn func() context.Context,
) *SftpService {
	service := &SftpService{
		sessions: sessions, terminal: terminal, transfers: transfers, context: contextFn,
		endpoints: make(map[string]*sftpx.Endpoint), dialSSH: sshx.Dial,
	}
	service.pickFiles = func(ctx context.Context) ([]string, error) {
		if value := os.Getenv("OPENTERMINAL_TEST_DIALOG_PATH"); value != "" {
			return splitDialogPaths(value), nil
		}
		return shell.OpenFiles(ctx, wruntime.OpenDialogOptions{Title: "Selecionar arquivos para enviar"})
	}
	service.pickDir = func(ctx context.Context) (string, error) {
		if value := os.Getenv("OPENTERMINAL_TEST_DIALOG_PATH"); value != "" {
			paths := splitDialogPaths(value)
			if len(paths) > 0 {
				return paths[0], nil
			}
			return "", nil
		}
		return shell.OpenDirectory(ctx, wruntime.OpenDialogOptions{
			Title:                "Selecionar pasta de destino",
			CanCreateDirectories: true,
		})
	}
	return service
}

func splitDialogPaths(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ';' || r == os.PathListSeparator })
	out := make([]string, 0, len(parts))
	for _, item := range parts {
		if clean := strings.TrimSpace(item); clean != "" {
			out = append(out, clean)
		}
	}
	return out
}

func (s *SftpService) Open(hostID string) (OpenSFTPResult, error) {
	if err := validate.UUID(hostID); err != nil {
		return OpenSFTPResult{}, clean(err)
	}
	if _, err := s.terminal.optionsForHost(hostID); err != nil {
		return OpenSFTPResult{}, clean(err)
	}
	id := domain.NewID()
	result := make(chan sftpOpenAttempt, 1)
	s.sessions.OpenID(id, session.KindSFTP, &hostID, s.dial(id, hostID, result))
	if err := s.sessions.Resize(id, 1, 1); err != nil {
		_ = s.sessions.Close(id)
		return OpenSFTPResult{}, clean(err)
	}
	attempt := <-result
	if attempt.err != nil {
		_ = s.sessions.Close(id)
		return OpenSFTPResult{}, clean(attempt.err)
	}
	return OpenSFTPResult{SessionID: id, HomePath: attempt.home}, nil
}

func (s *SftpService) List(sessionID, remotePath string) ([]sftpx.RemoteEntry, error) {
	endpoint, err := s.endpoint(sessionID, remotePath)
	if err != nil {
		return nil, clean(err)
	}
	entries, err := endpoint.List(remotePath)
	return entries, cleanSFTP(err)
}

func (s *SftpService) Mkdir(sessionID, remotePath string) error {
	endpoint, err := s.endpoint(sessionID, remotePath)
	if err != nil {
		return clean(err)
	}
	return cleanSFTP(endpoint.Mkdir(remotePath))
}

func (s *SftpService) Rename(sessionID, from, to string) error {
	if err := validRemotePath(to); err != nil {
		return clean(err)
	}
	endpoint, err := s.endpoint(sessionID, from)
	if err != nil {
		return clean(err)
	}
	return cleanSFTP(endpoint.Rename(from, to))
}

func (s *SftpService) Remove(sessionID, remotePath string, recursive bool) error {
	endpoint, err := s.endpoint(sessionID, remotePath)
	if err != nil {
		return clean(err)
	}
	return cleanSFTP(endpoint.Remove(remotePath, recursive))
}

func (s *SftpService) Upload(sessionID, remoteDir string) (TransferIDs, error) {
	endpoint, err := s.endpoint(sessionID, remoteDir)
	if err != nil {
		return TransferIDs{}, clean(err)
	}
	files, err := s.pickFiles(s.context())
	if err != nil {
		return TransferIDs{}, clean(err)
	}
	if len(files) == 0 {
		return TransferIDs{TransferIDs: []string{}}, nil
	}
	for index, file := range files {
		files[index] = filepath.Clean(file)
		if !filepath.IsAbs(files[index]) {
			return TransferIDs{}, apperr.New(apperr.InvalidInput)
		}
	}
	ids, err := s.transfers.Upload(endpoint, files, remoteDir)
	return TransferIDs{TransferIDs: ids}, cleanSFTP(err)
}

func (s *SftpService) Download(sessionID string, remotePaths []string) (TransferIDs, error) {
	if len(remotePaths) < 1 || len(remotePaths) > 1000 {
		return TransferIDs{}, apperr.New(apperr.InvalidInput)
	}
	for _, remotePath := range remotePaths {
		if err := validRemotePath(remotePath); err != nil {
			return TransferIDs{}, clean(err)
		}
	}
	endpoint, err := s.endpoint(sessionID, remotePaths[0])
	if err != nil {
		return TransferIDs{}, clean(err)
	}
	directory, err := s.pickDir(s.context())
	if err != nil {
		return TransferIDs{}, clean(err)
	}
	if directory == "" {
		return TransferIDs{TransferIDs: []string{}}, nil
	}
	directory = filepath.Clean(directory)
	if !filepath.IsAbs(directory) {
		return TransferIDs{}, apperr.New(apperr.InvalidInput)
	}
	ids, err := s.transfers.Download(endpoint, remotePaths, directory)
	return TransferIDs{TransferIDs: ids}, cleanSFTP(err)
}

func (s *SftpService) ResolveConflict(transferID string, choice sftpx.ConflictChoice) error {
	if err := validate.UUID(transferID); err != nil {
		return clean(err)
	}
	return clean(s.transfers.ResolveConflict(transferID, choice))
}

func (s *SftpService) Cancel(transferID string) error {
	if err := validate.UUID(transferID); err != nil {
		return clean(err)
	}
	return clean(s.transfers.Cancel(transferID))
}

func (s *SftpService) endpoint(sessionID, remotePath string) (*sftpx.Endpoint, error) {
	if err := validate.UUID(sessionID); err != nil {
		return nil, err
	}
	if err := validRemotePath(remotePath); err != nil {
		return nil, err
	}
	s.mu.RLock()
	endpoint := s.endpoints[sessionID]
	s.mu.RUnlock()
	if endpoint == nil {
		return nil, apperr.New(apperr.SessionClosed)
	}
	return endpoint, nil
}

func validRemotePath(remotePath string) error {
	if remotePath == "" || len(remotePath) > maxRemotePathBytes || !utf8.ValidString(remotePath) || strings.ContainsRune(remotePath, 0) {
		return apperr.New(apperr.InvalidInput)
	}
	return nil
}

func cleanSFTP(err error) error {
	if err == nil {
		return nil
	}
	code := apperr.CodeOf(err)
	if code == apperr.Internal {
		code = apperr.SftpError
	}
	return apperr.New(code)
}

type sftpOpenAttempt struct {
	home string
	err  error
}

func (s *SftpService) dial(sessionID, hostID string, result chan<- sftpOpenAttempt) session.Dialer {
	return func(ctx context.Context, interactor session.Interactor) (session.Conn, error) {
		options, err := s.terminal.optionsForHost(hostID)
		if err != nil {
			sendSFTPAttempt(result, sftpOpenAttempt{err: err})
			return nil, err
		}
		client, err := s.dialSSH(ctx, options, interactor)
		if err != nil {
			sendSFTPAttempt(result, sftpOpenAttempt{err: err})
			return nil, err
		}
		endpoint, err := sftpx.NewEndpoint(sessionID, client.Client)
		if err != nil {
			_ = client.Close()
			sendSFTPAttempt(result, sftpOpenAttempt{err: err})
			return nil, err
		}
		home, err := endpoint.RealPath(".")
		if err != nil {
			_ = endpoint.Close()
			_ = client.Close()
			sendSFTPAttempt(result, sftpOpenAttempt{err: err})
			return nil, err
		}
		connection := &sftpSessionConn{service: s, sessionID: sessionID, endpoint: endpoint, client: client}
		s.mu.Lock()
		s.endpoints[sessionID] = endpoint
		s.mu.Unlock()
		sendSFTPAttempt(result, sftpOpenAttempt{home: home})
		return connection, nil
	}
}

func sendSFTPAttempt(channel chan<- sftpOpenAttempt, attempt sftpOpenAttempt) {
	if channel == nil {
		return
	}
	channel <- attempt
}

type sftpSessionConn struct {
	service   *SftpService
	sessionID string
	endpoint  *sftpx.Endpoint
	client    *sshx.Client
	once      sync.Once
}

func (c *sftpSessionConn) Start(int, int, io.Writer) error { return nil }
func (c *sftpSessionConn) Wait() error                     { return c.client.Wait() }
func (c *sftpSessionConn) Write([]byte) error              { return apperr.New(apperr.InvalidInput) }
func (c *sftpSessionConn) Resize(int, int) error           { return nil }

func (c *sftpSessionConn) Close() error {
	var closeErr error
	c.once.Do(func() {
		c.service.transfers.CancelSession(c.sessionID)
		_ = c.endpoint.Close()
		closeErr = c.client.Close()
		c.service.mu.Lock()
		if c.service.endpoints[c.sessionID] == c.endpoint {
			delete(c.service.endpoints, c.sessionID)
		}
		c.service.mu.Unlock()
	})
	return closeErr
}
