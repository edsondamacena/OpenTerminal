package bridge

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"openterminal/internal/apperr"
	"openterminal/internal/domain"
	"openterminal/internal/model"
	"openterminal/internal/validate"
	"openterminal/internal/vault"
)

type PickedKeyFile struct {
	FilePath string `json:"filePath"`
}

type KeySource struct {
	FilePath *string `json:"filePath"`
	PEM      *string `json:"pem"`
}

type CredentialService struct {
	store   *vault.Store
	context func() context.Context

	mu     sync.Mutex
	picked map[string]struct{}

	pickFile func(context.Context) (string, error)
	copyText func(context.Context, string) error
}

func newCredentialService(store *vault.Store, contextFn func() context.Context) *CredentialService {
	return &CredentialService{
		store: store, context: contextFn, picked: map[string]struct{}{},
		pickFile: func(ctx context.Context) (string, error) {
			return shell.OpenFile(ctx, wruntime.OpenDialogOptions{
				Title: "Escolher chave privada",
				Filters: []wruntime.FileFilter{{
					DisplayName: "Chaves SSH",
					Pattern:     "*.pem;*.key;id_*;*",
				}},
			})
		},
		copyText: func(ctx context.Context, text string) error { return shell.ClipboardSetText(ctx, text) },
	}
}

func (s *CredentialService) List() ([]domain.CredentialSummary, error) {
	content, err := s.store.Read()
	if err != nil {
		return nil, clean(err)
	}
	out := make([]domain.CredentialSummary, 0, len(content.Credentials))
	for _, credential := range content.Credentials {
		out = append(out, domain.ToSummary(content, credential))
	}
	return out, nil
}

func (s *CredentialService) CreatePassword(name, password string) (domain.CredentialSummary, error) {
	var created model.Credential
	err := s.store.Mutate(func(content *model.Content) error {
		var err error
		created, err = domain.CreatePassword(content, name, password, domain.Now())
		return err
	})
	return s.summary(created, err)
}

func (s *CredentialService) PickKeyFile() (*PickedKeyFile, error) {
	path, err := s.pickFile(s.context())
	if err != nil {
		return nil, clean(err)
	}
	if path == "" {
		return nil, nil
	}
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return nil, apperr.New(apperr.InvalidInput)
	}
	s.mu.Lock()
	s.picked[path] = struct{}{}
	s.mu.Unlock()
	return &PickedKeyFile{FilePath: path}, nil
}

func (s *CredentialService) ImportKey(name string, source KeySource, passphrase *string) (domain.CredentialSummary, error) {
	if (source.FilePath == nil) == (source.PEM == nil) {
		return domain.CredentialSummary{}, apperr.New(apperr.InvalidInput)
	}
	pemText := ""
	if source.PEM != nil {
		if len(*source.PEM) > domain.MaxKeyBytes {
			return domain.CredentialSummary{}, apperr.New(apperr.KeyUnsupported)
		}
		pemText = *source.PEM
	} else {
		path := filepath.Clean(*source.FilePath)
		s.mu.Lock()
		_, allowed := s.picked[path]
		delete(s.picked, path)
		s.mu.Unlock()
		if !allowed {
			return domain.CredentialSummary{}, apperr.New(apperr.InvalidInput)
		}
		var err error
		pemText, err = readKeyFile(path)
		if err != nil {
			return domain.CredentialSummary{}, clean(err)
		}
	}
	passphrase = emptyToNil(passphrase)
	var created model.Credential
	err := s.store.Mutate(func(content *model.Content) error {
		var err error
		created, err = domain.ImportKey(content, name, pemText, passphrase, domain.Now())
		return err
	})
	return s.summary(created, err)
}

func (s *CredentialService) GenerateEd25519(name string, passphrase *string) (domain.CredentialSummary, error) {
	passphrase = emptyToNil(passphrase)
	var created model.Credential
	err := s.store.Mutate(func(content *model.Content) error {
		var err error
		created, err = domain.GenerateEd25519(content, name, passphrase, domain.Now())
		return err
	})
	return s.summary(created, err)
}

func (s *CredentialService) CopyPublicKey(id string) error {
	if err := validate.UUID(id); err != nil {
		return clean(err)
	}
	content, err := s.store.Read()
	if err != nil {
		return clean(err)
	}
	credential, err := domain.FindCredential(content, id)
	if err != nil {
		return clean(err)
	}
	if credential.Kind != model.CredentialKey {
		return apperr.New(apperr.InvalidInput)
	}
	return clean(s.copyText(s.context(), credential.PublicKey))
}

func (s *CredentialService) Rename(id, name string) (domain.CredentialSummary, error) {
	if err := validate.UUID(id); err != nil {
		return domain.CredentialSummary{}, clean(err)
	}
	var renamed model.Credential
	err := s.store.Mutate(func(content *model.Content) error {
		var err error
		renamed, err = domain.RenameCredential(content, id, name)
		return err
	})
	return s.summary(renamed, err)
}

func (s *CredentialService) Remove(id string) error {
	if err := validate.UUID(id); err != nil {
		return clean(err)
	}
	return clean(s.store.Mutate(func(content *model.Content) error {
		return domain.RemoveCredential(content, id)
	}))
}

func (s *CredentialService) summary(credential model.Credential, err error) (domain.CredentialSummary, error) {
	if err != nil {
		return domain.CredentialSummary{}, clean(err)
	}
	content, err := s.store.Read()
	if err != nil {
		return domain.CredentialSummary{}, clean(err)
	}
	return domain.ToSummary(content, credential), nil
}

func readKeyFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, domain.MaxKeyBytes+1))
	if err != nil {
		return "", err
	}
	if len(raw) > domain.MaxKeyBytes {
		return "", apperr.New(apperr.KeyUnsupported)
	}
	return string(raw), nil
}

func emptyToNil(value *string) *string {
	if value != nil && *value == "" {
		return nil
	}
	return value
}
