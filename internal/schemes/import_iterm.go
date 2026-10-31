package schemes

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"strings"

	"openterminal/internal/model"
)

type plistValue struct {
	kind string
	text string
	dict map[string]plistValue
}

func importITerm(data []byte, fallbackName string) (ImportResult, error) {
	root, err := parsePlist(data)
	if err != nil || root.kind != "dict" {
		return ImportResult{}, invalid("plist do iTerm2 malformado")
	}
	name := fallbackName
	if v, ok := root.dict["Name"]; ok && strings.TrimSpace(v.text) != "" {
		name = strings.TrimSpace(v.text)
	}
	if strings.TrimSpace(name) == "" {
		return ImportResult{}, invalid("o esquema não tem nome")
	}
	defaults := Presets()[0]
	completed := []string{}
	read := func(key, fallback string) (string, error) {
		v, ok := root.dict[key]
		if !ok {
			completed = append(completed, key)
			return fallback, nil
		}
		return itermColor(v, key)
	}
	fg, err := read("Foreground Color", defaults.Foreground)
	if err != nil {
		return ImportResult{}, err
	}
	bg, err := read("Background Color", defaults.Background)
	if err != nil {
		return ImportResult{}, err
	}
	cursor, err := read("Cursor Color", defaults.Cursor)
	if err != nil {
		return ImportResult{}, err
	}
	selection, err := read("Selection Color", defaults.Selection[:7])
	if err != nil {
		return ImportResult{}, err
	}
	ansi := make([]string, 16)
	for i := range ansi {
		ansi[i], err = read(fmt.Sprintf("Ansi %d Color", i), defaults.ANSI[i])
		if err != nil {
			return ImportResult{}, err
		}
	}
	s := model.ColorScheme{ID: newID(), Name: name, Origin: model.SchemeImported, Foreground: fg, Background: bg, Cursor: cursor, Selection: selection, ANSI: ansi}
	return ImportResult{Imported: []model.ColorScheme{s}, Completed: completed}, nil
}

func parsePlist(data []byte) (plistValue, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := d.Token()
		if err != nil {
			return plistValue{}, err
		}
		if start, ok := tok.(xml.StartElement); ok && start.Name.Local == "dict" {
			return parseDict(d)
		}
	}
}

func parseDict(d *xml.Decoder) (plistValue, error) {
	out := plistValue{kind: "dict", dict: map[string]plistValue{}}
	var key string
	for {
		tok, err := d.Token()
		if err != nil {
			return plistValue{}, err
		}
		switch x := tok.(type) {
		case xml.StartElement:
			switch x.Name.Local {
			case "key":
				key, err = readText(d, x)
			case "dict":
				var v plistValue
				v, err = parseDict(d)
				if key == "" {
					return plistValue{}, fmt.Errorf("dict sem key")
				}
				out.dict[key], key = v, ""
			case "string", "real", "integer":
				var text string
				text, err = readText(d, x)
				if key == "" {
					return plistValue{}, fmt.Errorf("valor sem key")
				}
				out.dict[key], key = plistValue{kind: x.Name.Local, text: text}, ""
			}
			if err != nil {
				return plistValue{}, err
			}
		case xml.EndElement:
			if x.Name.Local == "dict" {
				return out, nil
			}
		}
	}
}

func readText(d *xml.Decoder, start xml.StartElement) (string, error) {
	var b strings.Builder
	for {
		tok, err := d.Token()
		if err != nil {
			return "", err
		}
		switch x := tok.(type) {
		case xml.CharData:
			b.Write([]byte(x))
		case xml.EndElement:
			if x.Name == start.Name {
				return strings.TrimSpace(b.String()), nil
			}
		}
	}
}

func itermColor(v plistValue, key string) (string, error) {
	if v.kind != "dict" {
		return "", invalid(key + " não é um dicionário de cor")
	}
	component := func(name string) (float64, error) {
		item, ok := v.dict[name]
		if !ok {
			return 0, fmt.Errorf("ausente")
		}
		n, err := strconv.ParseFloat(item.text, 64)
		if err != nil || n < 0 || n > 1 {
			return 0, fmt.Errorf("inválido")
		}
		return n, nil
	}
	r, e1 := component("Red Component")
	g, e2 := component("Green Component")
	b, e3 := component("Blue Component")
	if e1 != nil || e2 != nil || e3 != nil {
		return "", invalid(key + " tem componentes inválidos")
	}
	space := strings.ToLower(v.dict["Color Space"].text)
	if strings.Contains(space, "p3") {
		r, g, b = p3ToSRGB(r, g, b)
	}
	return fmt.Sprintf("#%02x%02x%02x", toByte(r), toByte(g), toByte(b)), nil
}

func p3ToSRGB(r, g, b float64) (float64, float64, float64) {
	linear := func(x float64) float64 {
		if x <= .04045 {
			return x / 12.92
		}
		return math.Pow((x+.055)/1.055, 2.4)
	}
	encode := func(x float64) float64 {
		x = max(0, min(1, x))
		if x <= .0031308 {
			return 12.92 * x
		}
		return 1.055*math.Pow(x, 1/2.4) - .055
	}
	r, g, b = linear(r), linear(g), linear(b)
	return encode(1.224745*r - .224904*g), encode(-.042058*r + 1.042081*g), encode(-.019642*r - .078655*g + 1.098537*b)
}

func toByte(v float64) int { return int(math.Round(max(0, min(1, v)) * 255)) }
