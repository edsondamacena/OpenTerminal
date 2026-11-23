package bridge

import (
	"context"
	"net/http"
	"testing"

	"openterminal/internal/update"
)

func TestInstallUpdateDownloadsAndOpensInstaller(t *testing.T) {
	oldCheck, oldDownload, oldOpen := checkUpdate, downloadUpdate, openInstaller
	t.Cleanup(func() { checkUpdate, downloadUpdate, openInstaller = oldCheck, oldDownload, oldOpen })

	checkUpdate = func(context.Context, *http.Client, string, string, string) (update.Info, error) {
		return update.Info{Current: "3.2.2", Latest: "3.2.3", Available: true, AssetName: "OpenTerminal-3.2.3.apk", PageURL: "https://github.com/x"}, nil
	}
	downloadUpdate = func(context.Context, *http.Client, update.Info, string) (string, error) {
		return "/tmp/OpenTerminal-3.2.3.apk", nil
	}
	var installed string
	openInstaller = func(path string) error { installed = path; return nil }
	var opened string
	app := &AppService{context: context.Background, openURL: func(_ context.Context, url string) { opened = url }}

	res, err := app.InstallUpdate("3.2.2")
	if err != nil || !res.Started || res.OpenedPage || installed != "/tmp/OpenTerminal-3.2.3.apk" || opened != "" {
		t.Fatalf("res %+v err %v instalado %q página %q", res, err, installed, opened)
	}

	// Sem instalador (ex.: Android antigo sem a ponte): abre a página da Release.
	openInstaller = func(string) error { return context.Canceled }
	res, err = app.InstallUpdate("3.2.2")
	if err != nil || res.Started || !res.OpenedPage || opened != "https://github.com/x" {
		t.Fatalf("fallback %+v err %v página %q", res, err, opened)
	}
}
