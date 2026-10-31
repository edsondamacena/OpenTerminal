package bridge

import (
	"os"
	"path/filepath"
	"runtime"

	"openterminal/internal/apperr"
	"openterminal/internal/update"
)

// Botão Atualizar (31/10): procura a última Release no GitHub; o instalador só abre depois de
// conferir o SHA-256 contra o SHA256SUMS da mesma Release.

// UpdateResult é o que a tela mostra depois do clique em Atualizar.
type UpdateResult struct {
	update.Info
	Started    bool `json:"started"`    // o instalador foi aberto
	OpenedPage bool `json:"openedPage"` // sem instalador automático: abriu a página da Release
}

var (
	checkUpdate    = update.Check
	downloadUpdate = update.Download
	openInstaller  = update.Open
)

func (s *AppService) CheckUpdate(current string) (update.Info, error) {
	info, err := checkUpdate(s.context(), update.Client(), current, runtime.GOOS, runtime.GOARCH)
	return info, apperr.Sanitize(err)
}

func (s *AppService) InstallUpdate(current string) (UpdateResult, error) {
	ctx := s.context()
	info, err := checkUpdate(ctx, update.Client(), current, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return UpdateResult{Info: info}, apperr.Sanitize(err)
	}
	res := UpdateResult{Info: info}
	if !info.Available {
		return res, nil
	}
	if info.AssetName == "" || runtime.GOOS == "android" {
		s.openURL(ctx, info.PageURL)
		res.OpenedPage = true
		return res, nil
	}
	path, err := downloadUpdate(ctx, update.Client(), info, filepath.Join(os.TempDir(), "openterminal-update"))
	if err != nil {
		return res, apperr.Sanitize(err)
	}
	if err := openInstaller(path); err != nil {
		s.openURL(ctx, info.PageURL)
		res.OpenedPage = true
		return res, nil
	}
	res.Started = true
	return res, nil
}
