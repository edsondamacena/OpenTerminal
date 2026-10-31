// Package fonts discovers monospaced fonts available to the terminal.
package fonts

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

type Source string

const (
	SourceSystem  Source = "system"
	SourceBundled Source = "bundled"
)

type Font struct {
	Family    string `json:"family"`
	Source    Source `json:"source"`
	Ligatures bool   `json:"ligatures"`
}

var bundled = []Font{
	{Family: "JetBrains Mono", Source: SourceBundled, Ligatures: true},
	{Family: "Cascadia Code", Source: SourceBundled, Ligatures: true},
}

var listCache struct {
	sync.Once
	items []Font
	err   error
}

// List scans Windows font directories once per app process and returns a copy.
func List() ([]Font, error) {
	listCache.Do(func() {
		listCache.items, listCache.err = scan(defaultDirectories())
	})
	return append([]Font(nil), listCache.items...), listCache.err
}

func defaultDirectories() []string {
	windowsDir := os.Getenv("WINDIR")
	if windowsDir == "" {
		windowsDir = os.Getenv("SystemRoot")
	}
	dirs := make([]string, 0, 2)
	if windowsDir != "" {
		dirs = append(dirs, filepath.Join(windowsDir, "Fonts"))
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		dirs = append(dirs, filepath.Join(local, "Microsoft", "Windows", "Fonts"))
	}
	return dirs
}

func scan(dirs []string) ([]Font, error) {
	byFamily := make(map[string]Font, len(bundled)+32)
	for _, item := range bundled {
		byFamily[strings.ToLower(item.Family)] = item
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() || !fontExtension(entry.Name()) {
				continue
			}
			families := familiesInFile(filepath.Join(dir, entry.Name()))
			for _, family := range families {
				key := strings.ToLower(family)
				if _, exists := byFamily[key]; exists {
					continue
				}
				byFamily[key] = Font{Family: family, Source: SourceSystem, Ligatures: hasKnownLigatures(family)}
			}
		}
	}
	items := make([]Font, 0, len(byFamily))
	for _, item := range byFamily {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].Family) < strings.ToLower(items[j].Family)
	})
	return items, nil
}

func fontExtension(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".ttf", ".ttc", ".otf", ".otc":
		return true
	default:
		return false
	}
}

// familiesInFile lê só as tabelas que o sfnt pede: carregar os arquivos inteiros somava mais de
// 300 MB de heap (C:\Windows\Fonts) que o GC não devolvia com o app ocioso (SC-109).
func familiesInFile(path string) []string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	collection, err := sfnt.ParseCollectionReaderAt(file)
	if err != nil {
		return nil
	}
	var families []string
	for i := 0; i < collection.NumFonts(); i++ {
		f, err := collection.Font(i)
		if err != nil || !isMonospaced(f) {
			continue
		}
		family, err := f.Name(nil, sfnt.NameIDTypographicFamily)
		if err != nil || strings.TrimSpace(family) == "" {
			family, err = f.Name(nil, sfnt.NameIDFamily)
		}
		family = strings.TrimSpace(family)
		if err == nil && family != "" {
			families = append(families, family)
		}
	}
	return families
}

func isMonospaced(f *sfnt.Font) bool {
	var buffer sfnt.Buffer
	var advance fixed.Int26_6
	for i, r := range []rune("iWm0@#") {
		glyph, err := f.GlyphIndex(&buffer, r)
		if err != nil || glyph == 0 {
			return false
		}
		width, err := f.GlyphAdvance(&buffer, glyph, fixed.I(1000), font.HintingNone)
		if err != nil || width == 0 {
			return false
		}
		if i == 0 {
			advance = width
		} else if width != advance {
			return false
		}
	}
	return true
}

func hasKnownLigatures(family string) bool {
	name := strings.ToLower(family)
	return strings.Contains(name, "jetbrains mono") || strings.Contains(name, "cascadia code") ||
		strings.Contains(name, "fira code")
}
