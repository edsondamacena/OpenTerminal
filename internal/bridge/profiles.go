package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"openterminal/internal/apperr"
	"openterminal/internal/domain"
	"openterminal/internal/fonts"
	"openterminal/internal/model"
	"openterminal/internal/schemes"
	"openterminal/internal/session"
	"openterminal/internal/validate"
	"openterminal/internal/vault"
)

type ProfileChangedEvent struct {
	Scope  string  `json:"scope"`
	HostID *string `json:"hostId,omitempty"`
}

// TerminalProfilePatch mantém campos ausentes e permite limpar cursorColor com null.
type TerminalProfilePatch struct {
	Name              *string            `json:"name,omitempty"`
	FontFamily        *string            `json:"fontFamily,omitempty"`
	FontSize          *float64           `json:"fontSize,omitempty"`
	FontWeight        *int               `json:"fontWeight,omitempty"`
	FontWeightBold    *int               `json:"fontWeightBold,omitempty"`
	LineHeight        *float64           `json:"lineHeight,omitempty"`
	LetterSpacing     *float64           `json:"letterSpacing,omitempty"`
	Ligatures         *bool              `json:"ligatures,omitempty"`
	CursorStyle       *model.CursorStyle `json:"cursorStyle,omitempty"`
	CursorBlink       *bool              `json:"cursorBlink,omitempty"`
	Scrollback        *int               `json:"scrollback,omitempty"`
	BackgroundOpacity *float64           `json:"backgroundOpacity,omitempty"`
	Blur              *int               `json:"blur,omitempty"`
	Padding           *int               `json:"padding,omitempty"`
	ColorSchemeID     *string            `json:"colorSchemeId,omitempty"`

	CursorColor *string `json:"-"`
	cursorSet   bool
}

func (p *TerminalProfilePatch) UnmarshalJSON(data []byte) error {
	type plain TerminalProfilePatch
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*p = TerminalProfilePatch(decoded)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	raw, ok := fields["cursorColor"]
	if !ok {
		return nil
	}
	p.cursorSet = true
	if string(raw) == "null" {
		p.CursorColor = nil
		return nil
	}
	return json.Unmarshal(raw, &p.CursorColor)
}

func (p TerminalProfilePatch) apply(profile model.TerminalProfile) model.TerminalProfile {
	if p.Name != nil {
		profile.Name = *p.Name
	}
	if p.FontFamily != nil {
		profile.FontFamily = *p.FontFamily
	}
	if p.FontSize != nil {
		profile.FontSize = *p.FontSize
	}
	if p.FontWeight != nil {
		profile.FontWeight = *p.FontWeight
	}
	if p.FontWeightBold != nil {
		profile.FontWeightBold = *p.FontWeightBold
	}
	if p.LineHeight != nil {
		profile.LineHeight = *p.LineHeight
	}
	if p.LetterSpacing != nil {
		profile.LetterSpacing = *p.LetterSpacing
	}
	if p.Ligatures != nil {
		profile.Ligatures = *p.Ligatures
	}
	if p.CursorStyle != nil {
		profile.CursorStyle = *p.CursorStyle
	}
	if p.CursorBlink != nil {
		profile.CursorBlink = *p.CursorBlink
	}
	if p.cursorSet {
		profile.CursorColor = p.CursorColor
	}
	if p.Scrollback != nil {
		profile.Scrollback = *p.Scrollback
	}
	if p.BackgroundOpacity != nil {
		profile.BackgroundOpacity = *p.BackgroundOpacity
	}
	if p.Blur != nil {
		profile.Blur = *p.Blur
	}
	if p.Padding != nil {
		profile.Padding = *p.Padding
	}
	if p.ColorSchemeID != nil {
		profile.ColorSchemeID = *p.ColorSchemeID
	}
	return profile
}

type ProfileService struct {
	profiles *domain.Profiles
	sessions *session.Manager
	context  func() context.Context
	emit     func(string, any)

	pickImport func(context.Context) (string, error)
	pickExport func(context.Context, string) (string, error)
	listFonts  func() ([]fonts.Font, error)
}

func newProfileService(store *vault.Store, sessions *session.Manager, contextFn func() context.Context, emit func(string, any)) *ProfileService {
	return &ProfileService{
		profiles: domain.NewProfiles(store), sessions: sessions, context: contextFn, emit: emit,
		pickImport: func(ctx context.Context) (string, error) {
			if path := os.Getenv("OPENTERMINAL_TEST_DIALOG_PATH"); path != "" {
				return path, nil
			}
			return shell.OpenFile(ctx, wruntime.OpenDialogOptions{Title: "Importar esquema de cores",
				Filters: []wruntime.FileFilter{{DisplayName: "Esquemas de cores", Pattern: "*.json;*.itermcolors"}}})
		},
		pickExport: func(ctx context.Context, name string) (string, error) {
			if path := os.Getenv("OPENTERMINAL_TEST_DIALOG_PATH"); path != "" {
				return path, nil
			}
			return shell.SaveFile(ctx, wruntime.SaveDialogOptions{Title: "Exportar esquema de cores",
				DefaultFilename: safeSchemeFilename(name),
				Filters:         []wruntime.FileFilter{{DisplayName: "Esquema do Windows Terminal", Pattern: "*.json"}}})
		},
		listFonts: fonts.List,
	}
}

func (s *ProfileService) GetGlobal() (model.TerminalProfile, error) {
	profile, err := s.profiles.GetGlobal()
	return profile, clean(err)
}

func (s *ProfileService) UpdateGlobal(patch TerminalProfilePatch) (model.TerminalProfile, error) {
	current, err := s.profiles.GetGlobal()
	if err != nil {
		return model.TerminalProfile{}, clean(err)
	}
	updated, err := s.profiles.UpdateGlobal(patch.apply(current))
	if err == nil {
		s.emit("profiles:changed", ProfileChangedEvent{Scope: "global"})
	}
	return updated, clean(err)
}

func (s *ProfileService) GetForHost(hostID string) (*model.TerminalProfile, error) {
	profile, err := s.profiles.GetForHost(hostID)
	return profile, clean(err)
}

func (s *ProfileService) SetHostOverride(hostID string, profile *model.TerminalProfile) (*model.TerminalProfile, error) {
	updated, err := s.profiles.SetHostOverride(hostID, profile)
	if err == nil {
		s.emit("profiles:changed", ProfileChangedEvent{Scope: "host", HostID: &hostID})
	}
	return updated, clean(err)
}

func (s *ProfileService) Effective(sessionID string) (model.TerminalProfile, error) {
	if err := validate.UUID(sessionID); err != nil {
		return model.TerminalProfile{}, clean(err)
	}
	current, err := s.sessions.Get(sessionID)
	if err != nil {
		return model.TerminalProfile{}, clean(err)
	}
	if current.HostID != nil {
		profile, err := s.profiles.GetForHost(*current.HostID)
		if err != nil {
			return model.TerminalProfile{}, clean(err)
		}
		if profile != nil {
			return *profile, nil
		}
	}
	return s.GetGlobal()
}

func (s *ProfileService) ListSchemes() ([]model.ColorScheme, error) {
	items, err := s.profiles.ListSchemes()
	return items, clean(err)
}

func (s *ProfileService) SaveScheme(scheme model.ColorScheme) (model.ColorScheme, error) {
	updated, err := s.profiles.SaveScheme(scheme)
	if err == nil {
		s.emit("profiles:changed", ProfileChangedEvent{Scope: "global"})
	}
	return updated, clean(err)
}

func (s *ProfileService) DuplicateScheme(id, name string) (model.ColorScheme, error) {
	duplicate, err := s.profiles.DuplicateScheme(id, name)
	if err == nil {
		s.emit("profiles:changed", ProfileChangedEvent{Scope: "global"})
	}
	return duplicate, clean(err)
}

func (s *ProfileService) RemoveScheme(id string) error {
	err := s.profiles.RemoveScheme(id)
	if err == nil {
		s.emit("profiles:changed", ProfileChangedEvent{Scope: "global"})
	}
	return clean(err)
}

func (s *ProfileService) ImportScheme() (*schemes.ImportResult, error) {
	path, err := s.pickImport(s.context())
	if err != nil {
		return nil, clean(err)
	}
	if path == "" {
		return nil, nil
	}
	result, err := schemes.ImportFile(path)
	if err != nil {
		return nil, cleanSchemeError(err)
	}
	for i := range result.Imported {
		result.Imported[i], err = s.profiles.SaveScheme(result.Imported[i])
		if err != nil {
			return nil, clean(err)
		}
	}
	s.emit("profiles:changed", ProfileChangedEvent{Scope: "global"})
	return &result, nil
}

func (s *ProfileService) ExportScheme(id string) error {
	if err := validate.UUID(id); err != nil {
		return clean(err)
	}
	items, err := s.profiles.ListSchemes()
	if err != nil {
		return clean(err)
	}
	var selected *model.ColorScheme
	for i := range items {
		if items[i].ID == id {
			selected = &items[i]
			break
		}
	}
	if selected == nil {
		return apperr.New(apperr.NotFound)
	}
	path, err := s.pickExport(s.context(), selected.Name)
	if err != nil {
		return clean(err)
	}
	if path == "" {
		return nil
	}
	data, err := schemes.Export(*selected)
	if err != nil {
		return cleanSchemeError(err)
	}
	return clean(os.WriteFile(path, data, 0o600))
}

func (s *ProfileService) ListFonts() (items []fonts.Font, err error) {
	defer apperr.Recover("ProfileService.ListFonts", &err)
	items, err = s.listFonts()
	return items, clean(err)
}

func cleanSchemeError(err error) error {
	var appErr *apperr.Error
	if errors.As(err, &appErr) && appErr.Code == apperr.SchemeInvalid {
		return appErr
	}
	return clean(err)
}

func safeSchemeFilename(name string) string {
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 32 {
			return '_'
		}
		return r
	}, strings.TrimSpace(name))
	if name == "" {
		name = "esquema"
	}
	return filepath.Base(name) + ".json"
}
