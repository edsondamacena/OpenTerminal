package bridge

import (
	"encoding/json"
	"math"

	"openterminal/internal/apperr"
	"openterminal/internal/model"
	"openterminal/internal/validate"
	"openterminal/internal/vault"
	"openterminal/internal/vault/migrate001"
)

const erasePhrase = "APAGAR"

type VaultSettingsPatch map[string]any

type VaultService struct {
	store    *vault.Store
	autoLock *vault.AutoLock
	// O sync precisa da senha para derivar a chave da nuvem; ela não fica guardada.
	onUnlock         func(password string)
	onPasswordChange func(old, next string)
	onErase          func()
}

func newVaultService(store *vault.Store, autoLock *vault.AutoLock) *VaultService {
	return &VaultService{store: store, autoLock: autoLock}
}

func (s *VaultService) GetStatus() (vault.Status, error) {
	return s.store.Status(), nil
}

func (s *VaultService) Create(password, confirmation string) error {
	if err := s.store.Create(password, confirmation); err != nil {
		return clean(err)
	}
	s.unlocked(password)
	return nil
}

func (s *VaultService) Unlock(password string) error {
	before := s.store.Status().State
	var err error
	if before == vault.StateMigratable {
		err = migrate001.Migrate(s.store, password)
	} else {
		err = s.store.Unlock(password)
	}
	if err != nil {
		return clean(err)
	}
	if before != vault.StateUnlocked {
		s.unlocked(password)
	}
	return nil
}

func (s *VaultService) unlocked(password string) {
	if s.onUnlock != nil {
		s.onUnlock(password)
	}
}

func (s *VaultService) Lock() error {
	s.store.Lock()
	return nil
}

func (s *VaultService) ChangePassword(current, next, confirmation string) error {
	if err := s.store.ChangePassword(current, next, confirmation); err != nil {
		return clean(err)
	}
	if s.onPasswordChange != nil {
		s.onPasswordChange(current, next)
	}
	return nil
}

func (s *VaultService) Erase(confirmPhrase string) error {
	if confirmPhrase != erasePhrase {
		return apperr.New(apperr.InvalidInput)
	}
	if err := s.store.Erase(); err != nil {
		return clean(err)
	}
	if s.onErase != nil {
		s.onErase()
	}
	return nil
}

func (s *VaultService) GetSettings() (model.VaultSettings, error) {
	content, err := s.store.Read()
	if err != nil {
		return model.VaultSettings{}, clean(err)
	}
	return content.Settings, nil
}

func (s *VaultService) UpdateSettings(patch VaultSettingsPatch) (model.VaultSettings, error) {
	if patch == nil {
		return model.VaultSettings{}, apperr.New(apperr.InvalidInput)
	}
	var updated model.VaultSettings
	err := s.store.Mutate(func(content *model.Content) error {
		next := content.Settings
		for key, raw := range patch {
			switch key {
			case "autoLockMinutes":
				if raw == nil {
					next.AutoLockMinutes = nil
					continue
				}
				value, ok := bridgeInt(raw)
				if !ok || validate.Int(value, 1, 120) != nil {
					return apperr.New(apperr.InvalidInput)
				}
				next.AutoLockMinutes = model.Ptr(value)
			case "connectTimeoutSeconds":
				value, ok := bridgeInt(raw)
				if !ok || validate.Int(value, 5, 120) != nil {
					return apperr.New(apperr.InvalidInput)
				}
				next.ConnectTimeoutSeconds = value
			case "keepaliveSeconds":
				value, ok := bridgeInt(raw)
				if !ok || validate.Int(value, 10, 300) != nil {
					return apperr.New(apperr.InvalidInput)
				}
				next.KeepaliveSeconds = value
			case "legacyDefault":
				value, ok := raw.(bool)
				if !ok {
					return apperr.New(apperr.InvalidInput)
				}
				next.LegacyDefault = value
			case "term":
				value, ok := raw.(string)
				if !ok || !model.ValidTerm(value) {
					return apperr.New(apperr.InvalidInput)
				}
				next.Term = value
			default:
				return apperr.New(apperr.InvalidInput)
			}
		}
		content.Settings = next
		updated = next
		return nil
	})
	if err != nil {
		return model.VaultSettings{}, clean(err)
	}
	s.autoLock.Reconfigure()
	return updated, nil
}

func (s *VaultService) ReportActivity() error {
	s.autoLock.ReportActivity()
	return nil
}

func bridgeInt(value any) (int, bool) {
	switch number := value.(type) {
	case int:
		return number, true
	case int32:
		return int(number), true
	case int64:
		return int(number), int64(int(number)) == number
	case float64:
		if math.Trunc(number) == number && number >= -1<<31 && number <= 1<<31-1 {
			return int(number), true
		}
	case json.Number:
		parsed, err := number.Int64()
		if err == nil && int64(int(parsed)) == parsed {
			return int(parsed), true
		}
	}
	return 0, false
}
