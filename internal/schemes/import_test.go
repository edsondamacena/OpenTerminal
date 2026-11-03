package schemes

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"openterminal/internal/apperr"
	"openterminal/internal/model"
)

func fixture(name string) string {
	return filepath.Join("..", "..", "tests", "fixtures", "schemes", name)
}

func TestPresetsAreCompleteAndIndependent(t *testing.T) {
	got := Presets()
	if len(got) != 8 {
		t.Fatalf("esperava 8 presets, veio %d", len(got))
	}
	if brand := got[0]; brand.ID != BrandSchemeID || brand.Background != "#050505" || brand.Foreground != "#e8e8e8" ||
		brand.Cursor != "#f5f5f5" || brand.Selection != "#ffffff38" {
		t.Fatalf("preset da marca: %+v", brand)
	}
	seen := map[string]bool{}
	for _, s := range got {
		if seen[s.ID] || s.Origin != model.SchemeBuiltin || len(s.ANSI) != 16 {
			t.Fatalf("preset inválido: %+v", s)
		}
		seen[s.ID] = true
	}
	got[0].ANSI[0] = "#ffffff"
	if Presets()[0].ANSI[0] == "#ffffff" {
		t.Fatal("Presets deve devolver cópias independentes")
	}
}

func TestImportWindowsTerminalScheme(t *testing.T) {
	got, err := ImportFile(fixture("windows-terminal.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Imported) != 1 || len(got.Completed) != 0 {
		t.Fatalf("resultado inesperado: %+v", got)
	}
	wantANSI := []string{"#000000", "#cc0000", "#00cc00", "#cccc00", "#0000cc", "#cc00cc", "#00cccc", "#cccccc", "#555555", "#ff5555", "#55ff55", "#ffff55", "#5555ff", "#ff55ff", "#55ffff", "#ffffff"}
	s := got.Imported[0]
	if s.Name != "Fixture WT" || s.Origin != model.SchemeImported || s.Foreground != "#f0f0f0" ||
		s.Background != "#101010" || s.Cursor != "#abcdef" || s.Selection != "#22334488" ||
		!reflect.DeepEqual(s.ANSI, wantANSI) {
		t.Fatalf("esquema importado incorretamente: %+v", s)
	}
}

func TestImportWindowsTerminalSettings(t *testing.T) {
	got, err := ImportFile(fixture("windows-terminal-settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Imported) != 2 || got.Imported[0].Name != "Settings A" || got.Imported[1].Name != "Settings B" {
		t.Fatalf("lista de schemes não preservada: %+v", got.Imported)
	}
}

func TestImportITermSRGBAndDisplayP3(t *testing.T) {
	cases := []struct {
		file       string
		foreground string
	}{
		{"iterm-srgb.itermcolors", "#336699"},
		{"iterm-display-p3.itermcolors", "#db5e1f"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			got, err := ImportFile(fixture(tc.file))
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Imported) != 1 || got.Imported[0].Foreground != tc.foreground {
				t.Fatalf("conversão de cor incorreta: %+v", got)
			}
			if len(got.Imported[0].ANSI) != 16 {
				t.Fatalf("paleta ANSI incompleta: %+v", got.Imported[0].ANSI)
			}
		})
	}
}

func TestMissingColorsAreCompleted(t *testing.T) {
	got, err := ImportFile(fixture("missing-colors.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Imported) != 1 || len(got.Completed) == 0 {
		t.Fatalf("cores ausentes não foram informadas: %+v", got)
	}
	joined := strings.Join(got.Completed, ",")
	for _, field := range []string{"cursorColor", "selectionBackground", "red", "brightWhite"} {
		if !strings.Contains(joined, field) {
			t.Errorf("completed não contém %q: %v", field, got.Completed)
		}
	}
	if got.Imported[0].Cursor == "" || got.Imported[0].Selection == "" || len(got.Imported[0].ANSI) != 16 {
		t.Fatalf("resultado não foi completado: %+v", got.Imported[0])
	}
}

func TestInvalidSchemesReturnReason(t *testing.T) {
	for _, file := range []string{"invalid.json", "invalid.itermcolors"} {
		t.Run(file, func(t *testing.T) {
			_, err := ImportFile(fixture(file))
			if err == nil || apperr.CodeOf(err) != apperr.SchemeInvalid {
				t.Fatalf("esperava SCHEME_INVALID, veio %v", err)
			}
			var appErr *apperr.Error
			if !errors.As(err, &appErr) || strings.TrimSpace(appErr.Message) == "" {
				t.Fatalf("erro sem motivo seguro: %v", err)
			}
		})
	}
}

func TestExportAndReimportRoundTrip(t *testing.T) {
	first, err := ImportFile(fixture("windows-terminal.json"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Export(first.Imported[0])
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(b) {
		t.Fatalf("exportação não é JSON: %q", b)
	}
	second, err := Import(b, ".json", "roundtrip")
	if err != nil {
		t.Fatal(err)
	}
	want, got := first.Imported[0], second.Imported[0]
	want.ID, got.ID = "", ""
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round-trip divergente:\n got %+v\nwant %+v", got, want)
	}
}

func TestImportPlainArrayOfSchemes(t *testing.T) {
	data := []byte(`[
	  {"name":"A","foreground":"#ffffff","background":"#000000"},
	  {"name":"B","foreground":"#eeeeee","background":"#111111","selectionBackground":"#ffffff38"}
	]`)
	res, err := Import(data, ".json", "pacote")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Imported) != 2 || res.Imported[0].Name != "A" || res.Imported[1].Name != "B" {
		t.Fatalf("esperava A e B, veio %+v", res.Imported)
	}
	if _, err := Import([]byte(`[]`), ".json", "x"); err == nil {
		t.Fatal("lista vazia deveria falhar")
	}
}

func TestImportComboPackFromRepo(t *testing.T) {
	res, err := ImportFile(filepath.Join("..", "..", "temas", "OpenTerminal Combo.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Imported) < 100 {
		t.Fatalf("esperava mais de 100 esquemas, veio %d", len(res.Imported))
	}
	if len(res.Completed) != 0 {
		t.Fatalf("o pacote deveria vir completo, faltou: %v", res.Completed)
	}
}
