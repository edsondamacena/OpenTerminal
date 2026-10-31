package appwin

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Nomes da época do OpenHost (de onde o OpenTerminal veio). A pasta antiga nunca é apagada.
const (
	legacyDirName = "ShellVault"
	legacyDBName  = "shellvault.db"
	dbName        = "openterminal.db"
)

// migrateLegacyDir copia o diretório de dados do OpenHost para dir quando dir ainda não existe.
// A cópia vai para uma pasta temporária ao lado e só vira dir no rename final: uma cópia
// interrompida não deixa um cofre pela metade. O cache do WebView fica para trás.
func migrateLegacyDir(base, dir string) error {
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		return err
	}
	old := filepath.Join(base, legacyDirName)
	if info, err := os.Stat(old); err != nil || !info.IsDir() {
		return nil
	}
	tmp, err := os.MkdirTemp(base, ".openterminal-migracao-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	err = filepath.WalkDir(old, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(old, path)
		if rel == "webview" && d.IsDir() {
			return filepath.SkipDir
		}
		// shellvault.db, -wal e -shm: o WAL vai junto para não perder o que ainda não foi gravado.
		if name, ok := strings.CutPrefix(rel, legacyDBName); ok && !strings.ContainsRune(rel, filepath.Separator) {
			rel = dbName + name
		}
		target := filepath.Join(tmp, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		return copyFile(path, target)
	})
	if err != nil {
		return err
	}
	return os.Rename(tmp, dir)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
