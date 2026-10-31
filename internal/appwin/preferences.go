package appwin

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"openterminal/internal/localpty"
	"openterminal/internal/vault"
	"openterminal/internal/vault/migrate001"
)

const preferencesKey = "preferences"

var accentID = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

var shortcutActions = map[string]struct{}{
	"newTab": {}, "newPowerShell": {}, "closeTab": {}, "nextTab": {}, "prevTab": {},
	"search": {}, "palette": {}, "zoomIn": {}, "zoomOut": {}, "zoomReset": {},
	"toggleSidebar": {},
}

var (
	shortcutFunctionKey = regexp.MustCompile(`^F(?:[1-9]|1[0-9]|2[0-4])$`)
	shortcutCode        = regexp.MustCompile(`^(?:[A-Z]|Digit[0-9]|[A-Z][A-Za-z0-9]*)$`)
)

type WindowBounds struct {
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	X         *int `json:"x"`
	Y         *int `json:"y"`
	Maximized bool `json:"maximized"`
}

type Preferences struct {
	Theme              string            `json:"theme"`
	TerminalFontFamily string            `json:"terminalFontFamily"`
	TerminalFontSize   int               `json:"terminalFontSize"`
	Window             WindowBounds      `json:"window"`
	GlassEnabled       bool              `json:"glassEnabled"`
	SidebarCollapsed   bool              `json:"sidebarCollapsed"`
	Accent             string            `json:"accent"`
	DefaultShell       string            `json:"defaultShell"`
	CollapsedGroups    []string          `json:"collapsedGroups"`
	Shortcuts          map[string]string `json:"shortcuts"`
	CopyOnSelect       bool              `json:"copyOnSelect"`
	RightClickPaste    bool              `json:"rightClickPaste"`
	Bell               string            `json:"bell"`
	BoldBright         bool              `json:"boldBright"`
	// Highlight pinta palavras de rede (up/down, IPs, interfaces) em saídas sem cor própria.
	Highlight     bool `json:"highlight"`
	AutoReconnect bool `json:"autoReconnect"`
}

func DefaultPreferences() Preferences {
	return Preferences{
		Theme:              "system",
		TerminalFontFamily: "Cascadia Mono, Consolas, monospace",
		TerminalFontSize:   14,
		Window:             WindowBounds{Width: 1200, Height: 780},
		GlassEnabled:       true,
		SidebarCollapsed:   false,
		Accent:             "laranja",
		DefaultShell:       string(localpty.DefaultShell()),
		CollapsedGroups:    []string{},
		Shortcuts:          map[string]string{},
		CopyOnSelect:       false,
		RightClickPaste:    true,
		Bell:               "visual",
		BoldBright:         true,
		Highlight:          true,
		AutoReconnect:      false,
	}
}

type PreferenceStore struct {
	store *vault.Store
}

func NewPreferenceStore(store *vault.Store) *PreferenceStore {
	return &PreferenceStore{store: store}
}

func (s *PreferenceStore) Get() (Preferences, error) {
	preferences := DefaultPreferences()
	if ok, err := s.store.Preference(preferencesKey, &preferences); err != nil {
		return Preferences{}, err
	} else if ok {
		return normalizedPreferences(preferences), nil
	}

	var theme string
	if ok, err := s.store.Preference("theme", &theme); err != nil {
		return Preferences{}, err
	} else if ok {
		preferences.Theme = theme
	}
	var window WindowBounds
	if ok, err := s.store.Preference("window", &window); err != nil {
		return Preferences{}, err
	} else if ok {
		preferences.Window = window
	}
	var font migrate001.TerminalFont
	if ok, err := s.store.Preference(migrate001.PrefLegacyTerminalFont, &font); err != nil {
		return Preferences{}, err
	} else if ok {
		preferences.TerminalFontFamily = font.Family
		preferences.TerminalFontSize = font.Size
	}
	return normalizedPreferences(preferences), nil
}

func (s *PreferenceStore) Save(preferences Preferences) error {
	return s.store.SetPreference(preferencesKey, normalizedPreferences(preferences))
}

func normalizedPreferences(preferences Preferences) Preferences {
	defaults := DefaultPreferences()
	switch preferences.Theme {
	case "system", "light", "dark":
	default:
		preferences.Theme = defaults.Theme
	}
	font := strings.TrimSpace(preferences.TerminalFontFamily)
	if !utf8.ValidString(font) || utf8.RuneCountInString(font) < 1 || utf8.RuneCountInString(font) > 200 {
		font = defaults.TerminalFontFamily
	}
	preferences.TerminalFontFamily = font
	if preferences.TerminalFontSize < 8 || preferences.TerminalFontSize > 32 {
		preferences.TerminalFontSize = defaults.TerminalFontSize
	}
	if preferences.Window.Width < 400 || preferences.Window.Width > 10_000 ||
		preferences.Window.Height < 300 || preferences.Window.Height > 10_000 {
		preferences.Window = defaults.Window
	}
	if !localpty.ValidShellID(preferences.DefaultShell) {
		preferences.DefaultShell = defaults.DefaultShell
	}
	if !ValidAccent(preferences.Accent) {
		preferences.Accent = defaults.Accent
	}
	if preferences.CollapsedGroups == nil {
		preferences.CollapsedGroups = []string{}
	}
	if preferences.Shortcuts == nil || ValidateShortcuts(preferences.Shortcuts) != nil {
		preferences.Shortcuts = map[string]string{}
	}
	switch preferences.Bell {
	case "off", "sound", "visual":
	default:
		preferences.Bell = defaults.Bell
	}
	return preferences
}

func ValidAccent(value string) bool { return value == "system" || accentID.MatchString(value) }

// ValidateShortcuts valida somente as substituições gravadas. Uma combinação vazia desativa
// a ação e combinações ausentes continuam usando o padrão definido pela interface.
func ValidateShortcuts(shortcuts map[string]string) error {
	used := make(map[string]string, len(shortcuts))
	for action, combination := range shortcuts {
		if _, ok := shortcutActions[action]; !ok {
			return fmt.Errorf("ação de atalho desconhecida: %s", action)
		}
		if combination == "" {
			continue
		}
		if err := validateShortcutCombination(combination); err != nil {
			return fmt.Errorf("atalho de %s: %w", action, err)
		}
		if previous, exists := used[combination]; exists {
			return fmt.Errorf("combinação %s duplicada entre %s e %s", combination, previous, action)
		}
		used[combination] = action
	}
	return nil
}

func validateShortcutCombination(combination string) error {
	parts := strings.Split(combination, "+")
	if len(parts) == 0 || parts[len(parts)-1] == "" {
		return fmt.Errorf("combinação inválida: %s", combination)
	}
	code := parts[len(parts)-1]
	modifiers := parts[:len(parts)-1]
	wantModifier := 0
	hasCtrlOrAlt := false
	for _, modifier := range modifiers {
		position := 0
		switch modifier {
		case "Ctrl":
			position, hasCtrlOrAlt = 1, true
		case "Alt":
			position, hasCtrlOrAlt = 2, true
		case "Shift":
			position = 3
		default:
			return fmt.Errorf("modificador inválido em %s", combination)
		}
		if position <= wantModifier {
			return fmt.Errorf("modificadores fora da ordem Ctrl+Alt+Shift em %s", combination)
		}
		wantModifier = position
	}
	if !shortcutCode.MatchString(code) || strings.HasPrefix(code, "Key") {
		return fmt.Errorf("código de tecla inválido em %s", combination)
	}
	if !hasCtrlOrAlt && !shortcutFunctionKey.MatchString(code) {
		return fmt.Errorf("%s precisa de Ctrl ou Alt", combination)
	}
	return nil
}
