package bridge

import (
	"context"
	"net/url"

	"openterminal/internal/apperr"
	"openterminal/internal/appwin"
	"openterminal/internal/localpty"
	"openterminal/internal/validate"
)

type Appearance = appwin.Appearance

type PreferencesPatch map[string]any

type AppService struct {
	preferences *appwin.PreferenceStore
	appearance  *appwin.AppearanceMonitor
	closeGuard  *appwin.CloseGuard
	context     func() context.Context
	localShells func() []localpty.LocalShell

	minimise       func(context.Context)
	toggleMaximise func(context.Context)
	quit           func(context.Context)
	openURL        func(context.Context, string)
}

func newAppService(
	preferences *appwin.PreferenceStore,
	appearance *appwin.AppearanceMonitor,
	closeGuard *appwin.CloseGuard,
	contextFn func() context.Context,
) *AppService {
	return &AppService{
		preferences:    preferences,
		appearance:     appearance,
		closeGuard:     closeGuard,
		context:        contextFn,
		localShells:    localpty.ListShells,
		minimise:       func(ctx context.Context) { shell.Minimise(ctx) },
		toggleMaximise: func(ctx context.Context) { shell.ToggleMaximise(ctx) },
		quit:           func(ctx context.Context) { shell.Quit(ctx) },
		openURL:        func(ctx context.Context, url string) { shell.OpenURL(ctx, url) },
	}
}

func (s *AppService) GetAppearance() (Appearance, error) {
	return s.appearance.Current(), nil
}

func (s *AppService) GetPreferences() (appwin.Preferences, error) {
	preferences, err := s.preferences.Get()
	if err != nil {
		return appwin.Preferences{}, clean(err)
	}
	if preferences.DefaultShell != string(localpty.DefaultShell()) && !shellAvailable(s.localShells(), preferences.DefaultShell) {
		preferences.DefaultShell = string(localpty.DefaultShell())
		if err := s.preferences.Save(preferences); err != nil {
			return appwin.Preferences{}, clean(err)
		}
	}
	return preferences, nil
}

// ListLocalShells devolve os shells instalados para o menu de nova aba e para as preferências.
func (s *AppService) ListLocalShells() ([]localpty.LocalShell, error) {
	return s.localShells(), nil
}

func (s *AppService) UpdatePreferences(patch PreferencesPatch) (appwin.Preferences, error) {
	if patch == nil {
		return appwin.Preferences{}, apperr.New(apperr.InvalidInput)
	}
	preferences, err := s.preferences.Get()
	if err != nil {
		return appwin.Preferences{}, clean(err)
	}
	for key, raw := range patch {
		switch key {
		case "theme":
			value, ok := raw.(string)
			if !ok || validate.OneOf(value, "system", "light", "dark") != nil {
				return appwin.Preferences{}, apperr.New(apperr.InvalidInput)
			}
			preferences.Theme = value
		case "terminalFontFamily":
			value, ok := raw.(string)
			if !ok {
				return appwin.Preferences{}, apperr.New(apperr.InvalidInput)
			}
			value, err = validate.Text(value, 1, 200)
			if err != nil {
				return appwin.Preferences{}, clean(err)
			}
			preferences.TerminalFontFamily = value
		case "terminalFontSize":
			value, ok := bridgeInt(raw)
			if !ok || validate.Int(value, 8, 32) != nil {
				return appwin.Preferences{}, apperr.New(apperr.InvalidInput)
			}
			preferences.TerminalFontSize = value
		case "glassEnabled":
			value, ok := raw.(bool)
			if !ok {
				return appwin.Preferences{}, apperr.New(apperr.InvalidInput)
			}
			preferences.GlassEnabled = value
		case "sidebarCollapsed":
			value, ok := raw.(bool)
			if !ok {
				return appwin.Preferences{}, apperr.New(apperr.InvalidInput)
			}
			preferences.SidebarCollapsed = value
		case "accent":
			value, ok := raw.(string)
			if !ok || !appwin.ValidAccent(value) {
				return appwin.Preferences{}, apperr.New(apperr.InvalidInput)
			}
			preferences.Accent = value
		case "defaultShell":
			value, ok := raw.(string)
			if !ok || !localpty.ValidShellID(value) || !shellAvailable(s.localShells(), value) {
				return appwin.Preferences{}, apperr.New(apperr.InvalidInput)
			}
			preferences.DefaultShell = value
		case "collapsedGroups":
			groups, ok := bridgeStrings(raw)
			if !ok || len(groups) > 5000 {
				return appwin.Preferences{}, apperr.New(apperr.InvalidInput)
			}
			seen := make(map[string]bool, len(groups))
			preferences.CollapsedGroups = preferences.CollapsedGroups[:0]
			for _, id := range groups {
				if validate.UUID(id) != nil || seen[id] {
					return appwin.Preferences{}, apperr.New(apperr.InvalidInput)
				}
				seen[id] = true
				preferences.CollapsedGroups = append(preferences.CollapsedGroups, id)
			}
		case "shortcuts":
			shortcuts, ok := bridgeStringMap(raw)
			if !ok {
				return appwin.Preferences{}, invalidPreference("shortcuts precisa ser um objeto de texto para texto")
			}
			if err := appwin.ValidateShortcuts(shortcuts); err != nil {
				return appwin.Preferences{}, invalidPreference(err.Error())
			}
			preferences.Shortcuts = shortcuts
		case "copyOnSelect":
			value, ok := raw.(bool)
			if !ok {
				return appwin.Preferences{}, apperr.New(apperr.InvalidInput)
			}
			preferences.CopyOnSelect = value
		case "rightClickPaste":
			value, ok := raw.(bool)
			if !ok {
				return appwin.Preferences{}, apperr.New(apperr.InvalidInput)
			}
			preferences.RightClickPaste = value
		case "bell":
			value, ok := raw.(string)
			if !ok || validate.OneOf(value, "off", "sound", "visual") != nil {
				return appwin.Preferences{}, apperr.New(apperr.InvalidInput)
			}
			preferences.Bell = value
		case "boldBright":
			value, ok := raw.(bool)
			if !ok {
				return appwin.Preferences{}, apperr.New(apperr.InvalidInput)
			}
			preferences.BoldBright = value
		case "highlight":
			value, ok := raw.(bool)
			if !ok {
				return appwin.Preferences{}, apperr.New(apperr.InvalidInput)
			}
			preferences.Highlight = value
		case "autoReconnect":
			value, ok := raw.(bool)
			if !ok {
				return appwin.Preferences{}, apperr.New(apperr.InvalidInput)
			}
			preferences.AutoReconnect = value
		case "backspace":
			value, ok := raw.(string)
			if !ok || !appwin.ValidBackspace(value) {
				return appwin.Preferences{}, apperr.New(apperr.InvalidInput)
			}
			preferences.Backspace = value
		case "deleteKey":
			value, ok := raw.(string)
			if !ok || !appwin.ValidDeleteKey(value) {
				return appwin.Preferences{}, apperr.New(apperr.InvalidInput)
			}
			preferences.DeleteKey = value
		default:
			return appwin.Preferences{}, apperr.New(apperr.InvalidInput)
		}
	}
	if err := s.preferences.Save(preferences); err != nil {
		return appwin.Preferences{}, clean(err)
	}
	s.appearance.EmitCurrent()
	return preferences, nil
}

func shellAvailable(shells []localpty.LocalShell, id string) bool {
	for _, shell := range shells {
		if shell.ID == id {
			return true
		}
	}
	return false
}

func (s *AppService) ConfirmClose(confirmed bool) error {
	s.closeGuard.Confirm(confirmed)
	return nil
}

func (s *AppService) WindowMinimise() error {
	s.minimise(s.context())
	return nil
}

func (s *AppService) WindowToggleMaximise() error {
	s.toggleMaximise(s.context())
	return nil
}

func (s *AppService) WindowClose() error {
	if !s.closeGuard.Intercept() {
		s.quit(s.context())
	}
	return nil
}

func (s *AppService) OpenExternalUrl(rawURL string) error {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return apperr.New(apperr.InvalidInput)
	}
	s.openURL(s.context(), parsed.String())
	return nil
}

func bridgeStrings(value any) ([]string, bool) {
	switch values := value.(type) {
	case []string:
		return append([]string{}, values...), true
	case []any:
		out := make([]string, len(values))
		for i, value := range values {
			text, ok := value.(string)
			if !ok {
				return nil, false
			}
			out[i] = text
		}
		return out, true
	default:
		return nil, false
	}
}

func bridgeStringMap(value any) (map[string]string, bool) {
	switch values := value.(type) {
	case map[string]string:
		return cloneStringMap(values), true
	case map[string]any:
		out := make(map[string]string, len(values))
		for key, value := range values {
			text, ok := value.(string)
			if !ok {
				return nil, false
			}
			out[key] = text
		}
		return out, true
	default:
		return nil, false
	}
}

func cloneStringMap(values map[string]string) map[string]string {
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func invalidPreference(detail string) error {
	return clean(&apperr.Error{
		Code:    apperr.InvalidInput,
		Message: apperr.Messages[apperr.InvalidInput],
		Detail:  detail,
	})
}
