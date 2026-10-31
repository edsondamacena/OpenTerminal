// Package schemes importa, exporta e fornece presets de esquemas de cores.
package schemes

import "openterminal/internal/model"

const BrandSchemeID = "00000000-0000-4000-8000-000000000001"

// Presets devolve cópias dos esquemas embutidos para que chamadores possam alterá-las.
func Presets() []model.ColorScheme {
	return []model.ColorScheme{
		preset(BrandSchemeID, "OpenTerminal Dark", "#e8e8e8", "#050505", "#f5f5f5", "#ffffff38",
			"#1d2026", "#ff7b72", "#5cc98b", "#e8c35a", "#6aa8f0", "#c792ea", "#56c8d8", "#cdd2da", "#5c6370", "#ffa198", "#8ee0ae", "#f5d77f", "#9cc6ff", "#dcb2f5", "#8ce0ec", "#f6f8fa"), // TODO(T081): confirmar com os tokens finais da marca.
		preset("00000000-0000-4000-8000-000000000003", "Dracula", "#f8f8f2", "#282a36", "#f8f8f2", "#44475a",
			"#000000", "#ff5555", "#50fa7b", "#f1fa8c", "#bd93f9", "#ff79c6", "#8be9fd", "#bfbfbf", "#4d4d4d", "#ff6e67", "#5af78e", "#f4f99d", "#caa9fa", "#ff92d0", "#9aedfe", "#e6e6e6"),
		preset("00000000-0000-4000-8000-000000000004", "Nord", "#d8dee9", "#2e3440", "#d8dee9", "#434c5e",
			"#2e3440", "#bf616a", "#a3be8c", "#ebcb8b", "#5e81ac", "#b48ead", "#88c0d0", "#e5e9f0", "#4c566a", "#bf616a", "#a3be8c", "#ebcb8b", "#81a1c1", "#b48ead", "#8fbcbb", "#eceff4"),
		preset("00000000-0000-4000-8000-000000000005", "Solarized Dark", "#839496", "#002b36", "#93a1a1", "#073642",
			"#002b36", "#dc322f", "#859900", "#b58900", "#268bd2", "#d33682", "#2aa198", "#eee8d5", "#073642", "#cb4b16", "#586e75", "#657b83", "#839496", "#6c71c4", "#93a1a1", "#fdf6e3"),
		preset("00000000-0000-4000-8000-000000000006", "Solarized Light", "#657b83", "#fdf6e3", "#586e75", "#eee8d5",
			"#002b36", "#dc322f", "#859900", "#b58900", "#268bd2", "#d33682", "#2aa198", "#eee8d5", "#073642", "#cb4b16", "#586e75", "#657b83", "#839496", "#6c71c4", "#93a1a1", "#fdf6e3"),
		preset("00000000-0000-4000-8000-000000000007", "One Dark", "#abb2bf", "#282c34", "#528bff", "#3e4451",
			"#1e2127", "#e06c75", "#98c379", "#d19a66", "#61afef", "#c678dd", "#56b6c2", "#abb2bf", "#5c6370", "#e06c75", "#98c379", "#d19a66", "#61afef", "#c678dd", "#56b6c2", "#ffffff"),
		preset("00000000-0000-4000-8000-000000000008", "Tokyo Night", "#a9b1d6", "#1a1b26", "#c0caf5", "#33467c",
			"#32344a", "#f7768e", "#9ece6a", "#e0af68", "#7aa2f7", "#ad8ee6", "#449dab", "#787c99", "#444b6a", "#ff7a93", "#b9f27c", "#ff9e64", "#7da6ff", "#bb9af7", "#0db9d7", "#acb0d0"),
		preset("00000000-0000-4000-8000-000000000009", "Catppuccin Mocha", "#cdd6f4", "#1e1e2e", "#f5e0dc", "#585b70",
			"#45475a", "#f38ba8", "#a6e3a1", "#f9e2af", "#89b4fa", "#f5c2e7", "#94e2d5", "#bac2de", "#585b70", "#f38ba8", "#a6e3a1", "#f9e2af", "#89b4fa", "#f5c2e7", "#94e2d5", "#a6adc8"),
	}
}

func preset(id, name, foreground, background, cursor, selection string, ansi ...string) model.ColorScheme {
	return model.ColorScheme{ID: id, Name: name, Origin: model.SchemeBuiltin, Foreground: foreground,
		Background: background, Cursor: cursor, Selection: selection, ANSI: ansi}
}
