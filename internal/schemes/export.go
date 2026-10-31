package schemes

import (
	"encoding/json"
	"fmt"

	"openterminal/internal/model"
)

func Export(s model.ColorScheme) ([]byte, error) {
	if len(s.ANSI) != 16 {
		return nil, invalid("a paleta ANSI precisa ter 16 cores")
	}
	out := map[string]string{
		"name": s.Name, "foreground": s.Foreground, "background": s.Background,
		"cursorColor": s.Cursor, "selectionBackground": s.Selection,
	}
	for i, key := range wtANSIKeys {
		out[key] = s.ANSI[i]
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("exportar esquema: %w", err)
	}
	return append(b, '\n'), nil
}
