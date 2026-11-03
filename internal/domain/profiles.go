package domain

import (
	"fmt"
	"math"
	"regexp"

	"openterminal/internal/apperr"
	"openterminal/internal/model"
	"openterminal/internal/schemes"
	"openterminal/internal/validate"
)

var (
	colorRGB  = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	colorRGBA = regexp.MustCompile(`^#[0-9a-fA-F]{8}$`)
)

type profileStore interface {
	GlobalProfile() (model.TerminalProfile, error)
	SaveGlobalProfile(model.TerminalProfile) error
	HostProfile(string) (*model.TerminalProfile, error)
	SetHostProfile(string, *model.TerminalProfile) error
	ColorSchemes() ([]model.ColorScheme, error)
	SaveColorScheme(model.ColorScheme) error
	DeleteColorScheme(string) error
}

type Profiles struct{ store profileStore }

func NewProfiles(store profileStore) *Profiles { return &Profiles{store: store} }

func finiteRange(v, min, max float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= min && v <= max
}

func validateProfile(p model.TerminalProfile, schemes []model.ColorScheme) error {
	if validate.UUID(p.ID) != nil {
		return invalid()
	}
	if _, err := validate.Text(p.Name, 1, 100); err != nil {
		return err
	}
	if _, err := validate.Text(p.FontFamily, 1, 100); err != nil {
		return err
	}
	if !finiteRange(p.FontSize, 8, 32) || validate.Int(p.FontWeight, 100, 900) != nil ||
		validate.Int(p.FontWeightBold, 100, 900) != nil || !finiteRange(p.LineHeight, 1, 2) ||
		!finiteRange(p.LetterSpacing, -2, 10) || validate.Int(p.Scrollback, 100, 100_000) != nil ||
		!finiteRange(p.BackgroundOpacity, .5, 1) || validate.Int(p.Blur, 0, 40) != nil ||
		validate.Int(p.Padding, 0, 32) != nil ||
		validate.OneOf(p.CursorStyle, model.CursorBlock, model.CursorBar, model.CursorUnderline) != nil {
		return invalid()
	}
	if p.CursorColor != nil && !colorRGB.MatchString(*p.CursorColor) {
		return invalid()
	}
	for _, scheme := range schemes {
		if scheme.ID == p.ColorSchemeID {
			return nil
		}
	}
	return invalid()
}

func validateScheme(s model.ColorScheme) error {
	if validate.UUID(s.ID) != nil {
		return invalid()
	}
	if _, err := validate.Text(s.Name, 1, 100); err != nil {
		return err
	}
	if validate.OneOf(s.Origin, model.SchemeBuiltin, model.SchemeImported, model.SchemeCustom) != nil ||
		!colorRGB.MatchString(s.Foreground) || !colorRGB.MatchString(s.Background) ||
		!colorRGB.MatchString(s.Cursor) || (!colorRGB.MatchString(s.Selection) && !colorRGBA.MatchString(s.Selection)) ||
		len(s.ANSI) != 16 {
		return invalid()
	}
	for _, color := range s.ANSI {
		if !colorRGB.MatchString(color) {
			return invalid()
		}
	}
	return nil
}

func (p *Profiles) GetGlobal() (model.TerminalProfile, error) { return p.store.GlobalProfile() }

func (p *Profiles) UpdateGlobal(profile model.TerminalProfile) (model.TerminalProfile, error) {
	current, err := p.store.GlobalProfile()
	if err != nil {
		return model.TerminalProfile{}, err
	}
	profile.ID, profile.IsGlobal = current.ID, true
	items, err := p.store.ColorSchemes()
	if err != nil {
		return model.TerminalProfile{}, err
	}
	if err := validateProfile(profile, items); err != nil {
		return model.TerminalProfile{}, err
	}
	if err := p.store.SaveGlobalProfile(profile); err != nil {
		return model.TerminalProfile{}, err
	}
	return profile, nil
}

func (p *Profiles) GetForHost(hostID string) (*model.TerminalProfile, error) {
	if validate.UUID(hostID) != nil {
		return nil, invalid()
	}
	return p.store.HostProfile(hostID)
}

func (p *Profiles) SetHostOverride(hostID string, profile *model.TerminalProfile) (*model.TerminalProfile, error) {
	if validate.UUID(hostID) != nil {
		return nil, invalid()
	}
	if profile == nil {
		return nil, p.store.SetHostProfile(hostID, nil)
	}
	copy := *profile
	current, err := p.store.HostProfile(hostID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		copy.ID = NewID()
	} else {
		copy.ID = current.ID
	}
	copy.IsGlobal = false
	schemes, err := p.store.ColorSchemes()
	if err != nil {
		return nil, err
	}
	if err := validateProfile(copy, schemes); err != nil {
		return nil, err
	}
	if err := p.store.SetHostProfile(hostID, &copy); err != nil {
		return nil, err
	}
	return &copy, nil
}

func (p *Profiles) ListSchemes() ([]model.ColorScheme, error) { return p.store.ColorSchemes() }

func (p *Profiles) SaveScheme(scheme model.ColorScheme) (model.ColorScheme, error) {
	items, err := p.store.ColorSchemes()
	if err != nil {
		return model.ColorScheme{}, err
	}
	for _, existing := range items {
		if existing.ID == scheme.ID && existing.Origin == model.SchemeBuiltin {
			return model.ColorScheme{}, invalid()
		}
	}
	if scheme.Origin == model.SchemeBuiltin {
		return model.ColorScheme{}, invalid()
	}
	if scheme.ID == "" {
		scheme.ID = NewID()
	}
	if err := validateScheme(scheme); err != nil {
		return model.ColorScheme{}, err
	}
	if schemeNameTaken(items, scheme.ID, scheme.Name) {
		if scheme.Origin != model.SchemeImported {
			return model.ColorScheme{}, schemeAlreadyExists(scheme.Name)
		}
		scheme.Name = availableSchemeName(items, scheme.Name)
	}
	if err := p.store.SaveColorScheme(scheme); err != nil {
		return model.ColorScheme{}, err
	}
	return scheme, nil
}

func schemeNameTaken(items []model.ColorScheme, exceptID, name string) bool {
	for _, item := range items {
		if item.ID != exceptID && item.Name == name {
			return true
		}
	}
	return false
}

func availableSchemeName(items []model.ColorScheme, base string) string {
	for number := 2; ; number++ {
		suffix := fmt.Sprintf(" (%d)", number)
		runes := []rune(base)
		if limit := 100 - len([]rune(suffix)); len(runes) > limit {
			runes = runes[:limit]
		}
		candidate := string(runes) + suffix
		if !schemeNameTaken(items, "", candidate) {
			return candidate
		}
	}
}

func schemeAlreadyExists(name string) error {
	return &apperr.Error{
		Code: apperr.AlreadyExists, Message: apperr.Messages[apperr.AlreadyExists],
		Detail: fmt.Sprintf("já existe um esquema com o nome %q", name),
	}
}

func (p *Profiles) DuplicateScheme(id, name string) (model.ColorScheme, error) {
	if validate.UUID(id) != nil {
		return model.ColorScheme{}, invalid()
	}
	items, err := p.store.ColorSchemes()
	if err != nil {
		return model.ColorScheme{}, err
	}
	for _, item := range items {
		if item.ID == id {
			item.ID, item.Name, item.Origin = NewID(), name, model.SchemeCustom
			item.ANSI = append([]string(nil), item.ANSI...)
			return p.SaveScheme(item)
		}
	}
	return model.ColorScheme{}, apperr.New(apperr.NotFound)
}

func (p *Profiles) RemoveScheme(id string) error {
	if validate.UUID(id) != nil {
		return invalid()
	}
	items, err := p.store.ColorSchemes()
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.ID == id {
			if item.Origin == model.SchemeBuiltin {
				return invalid()
			}
			// O esquema aplicado no perfil global não trava a exclusão: o global volta para o
			// esquema da marca. Perfis próprios de host continuam bloqueando (InUse).
			global, err := p.store.GlobalProfile()
			if err != nil {
				return err
			}
			if global.ColorSchemeID == id {
				global.ColorSchemeID = schemes.BrandSchemeID
				if _, err := p.UpdateGlobal(global); err != nil {
					return err
				}
			}
			return p.store.DeleteColorScheme(id)
		}
	}
	return apperr.New(apperr.NotFound)
}
