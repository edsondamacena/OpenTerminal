package schemes

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"openterminal/internal/apperr"
	"openterminal/internal/model"
)

type ImportResult struct {
	Imported  []model.ColorScheme `json:"imported"`
	Completed []string            `json:"completed"`
}

var colorPattern = regexp.MustCompile(`(?i)^#[0-9a-f]{6}([0-9a-f]{2})?$`)

var wtANSIKeys = []string{"black", "red", "green", "yellow", "blue", "purple", "cyan", "white", "brightBlack", "brightRed", "brightGreen", "brightYellow", "brightBlue", "brightPurple", "brightCyan", "brightWhite"}

type wtScheme struct {
	Name                string  `json:"name"`
	Foreground          *string `json:"foreground"`
	Background          *string `json:"background"`
	CursorColor         *string `json:"cursorColor"`
	SelectionBackground *string `json:"selectionBackground"`
	Black               *string `json:"black"`
	Red                 *string `json:"red"`
	Green               *string `json:"green"`
	Yellow              *string `json:"yellow"`
	Blue                *string `json:"blue"`
	Purple              *string `json:"purple"`
	Cyan                *string `json:"cyan"`
	White               *string `json:"white"`
	BrightBlack         *string `json:"brightBlack"`
	BrightRed           *string `json:"brightRed"`
	BrightGreen         *string `json:"brightGreen"`
	BrightYellow        *string `json:"brightYellow"`
	BrightBlue          *string `json:"brightBlue"`
	BrightPurple        *string `json:"brightPurple"`
	BrightCyan          *string `json:"brightCyan"`
	BrightWhite         *string `json:"brightWhite"`
}

func ImportFile(path string) (ImportResult, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return ImportResult{}, err
	}
	return Import(b, filepath.Ext(path), strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
}

func Import(data []byte, extension, fallbackName string) (ImportResult, error) {
	if strings.EqualFold(extension, ".itermcolors") || looksLikePlist(data) {
		return importITerm(data, fallbackName)
	}
	return importWindowsTerminal(data, fallbackName)
}

func importWindowsTerminal(data []byte, fallbackName string) (ImportResult, error) {
	var inputs []wtScheme
	// Lista pura ([{...}, {...}]): um pacote de esquemas, como o OpenTerminal Combo.
	if trimmed := strings.TrimSpace(string(data)); strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal(data, &inputs); err != nil || len(inputs) == 0 {
			return ImportResult{}, invalid("a lista de esquemas está vazia ou é inválida")
		}
		return completeAll(inputs, fallbackName)
	}
	var envelope struct {
		Schemes json.RawMessage `json:"schemes"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return ImportResult{}, invalid("JSON malformado")
	}
	if len(envelope.Schemes) > 0 && string(envelope.Schemes) != "null" {
		if err := json.Unmarshal(envelope.Schemes, &inputs); err != nil || len(inputs) == 0 {
			return ImportResult{}, invalid("a lista schemes está vazia ou é inválida")
		}
	} else {
		var one wtScheme
		if err := json.Unmarshal(data, &one); err != nil {
			return ImportResult{}, invalid("esquema do Windows Terminal inválido")
		}
		inputs = []wtScheme{one}
	}
	return completeAll(inputs, fallbackName)
}

func completeAll(inputs []wtScheme, fallbackName string) (ImportResult, error) {
	result := ImportResult{Imported: make([]model.ColorScheme, 0, len(inputs)), Completed: []string{}}
	for i, input := range inputs {
		name := strings.TrimSpace(input.Name)
		if name == "" && len(inputs) == 1 {
			name = strings.TrimSpace(fallbackName)
		}
		if name == "" {
			return ImportResult{}, invalid(fmt.Sprintf("o esquema %d não tem nome", i+1))
		}
		scheme, completed, err := completeWT(input, name)
		if err != nil {
			return ImportResult{}, err
		}
		result.Imported = append(result.Imported, scheme)
		result.Completed = append(result.Completed, completed...)
	}
	return result, nil
}

func completeWT(w wtScheme, name string) (model.ColorScheme, []string, error) {
	defaults := Presets()[0]
	completed := []string{}
	get := func(key string, value *string, fallback string, alpha bool) (string, error) {
		if value == nil || strings.TrimSpace(*value) == "" {
			completed = append(completed, key)
			return fallback, nil
		}
		v := strings.ToLower(strings.TrimSpace(*value))
		if !colorPattern.MatchString(v) || (!alpha && len(v) != 7) {
			return "", invalid(key + " não é uma cor #rrggbb válida")
		}
		return v, nil
	}
	fg, err := get("foreground", w.Foreground, defaults.Foreground, false)
	if err != nil {
		return model.ColorScheme{}, nil, err
	}
	bg, err := get("background", w.Background, defaults.Background, false)
	if err != nil {
		return model.ColorScheme{}, nil, err
	}
	cursor, err := get("cursorColor", w.CursorColor, defaults.Cursor, false)
	if err != nil {
		return model.ColorScheme{}, nil, err
	}
	selection, err := get("selectionBackground", w.SelectionBackground, defaults.Selection, true)
	if err != nil {
		return model.ColorScheme{}, nil, err
	}
	values := []*string{w.Black, w.Red, w.Green, w.Yellow, w.Blue, w.Purple, w.Cyan, w.White, w.BrightBlack, w.BrightRed, w.BrightGreen, w.BrightYellow, w.BrightBlue, w.BrightPurple, w.BrightCyan, w.BrightWhite}
	ansi := make([]string, 16)
	for i := range ansi {
		ansi[i], err = get(wtANSIKeys[i], values[i], defaults.ANSI[i], false)
		if err != nil {
			return model.ColorScheme{}, nil, err
		}
	}
	return model.ColorScheme{ID: newID(), Name: name, Origin: model.SchemeImported, Foreground: fg, Background: bg, Cursor: cursor, Selection: selection, ANSI: ansi}, completed, nil
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand indisponível: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func invalid(reason string) error {
	return &apperr.Error{Code: apperr.SchemeInvalid, Message: "Esquema inválido: " + reason + "."}
}

func looksLikePlist(data []byte) bool {
	trimmed := strings.TrimSpace(string(data))
	return strings.HasPrefix(trimmed, "<?xml") || strings.HasPrefix(trimmed, "<plist")
}
