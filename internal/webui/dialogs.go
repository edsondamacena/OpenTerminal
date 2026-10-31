package webui

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Diálogos de arquivo pelo zenity (GTK, presente no Mint/Ubuntu/GNOME). Cancelar devolve "",
// como no Wails.

func zenity(ctx context.Context, args ...string) (string, error) {
	path, err := exec.LookPath("zenity")
	if err != nil {
		return "", errors.New("instale o zenity para escolher arquivos (sudo apt install zenity)")
	}
	out, err := exec.CommandContext(ctx, path, append([]string{"--file-selection"}, args...)...).Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

func dialogArgs(title, dir string, filters []wruntime.FileFilter) []string {
	args := []string{"--title=" + title}
	if dir != "" {
		args = append(args, "--filename="+dir+"/")
	}
	for _, f := range filters {
		args = append(args, "--file-filter="+f.DisplayName+" | "+strings.ReplaceAll(f.Pattern, ";", " "))
	}
	return args
}

func openFile(ctx context.Context, o wruntime.OpenDialogOptions) (string, error) {
	return zenity(ctx, dialogArgs(o.Title, o.DefaultDirectory, o.Filters)...)
}

func openFiles(ctx context.Context, o wruntime.OpenDialogOptions) ([]string, error) {
	out, err := zenity(ctx, append(dialogArgs(o.Title, o.DefaultDirectory, o.Filters), "--multiple", "--separator=\n")...)
	if err != nil || out == "" {
		return nil, err
	}
	return strings.Split(out, "\n"), nil
}

func openDirectory(ctx context.Context, o wruntime.OpenDialogOptions) (string, error) {
	return zenity(ctx, append(dialogArgs(o.Title, o.DefaultDirectory, nil), "--directory")...)
}

func saveFile(ctx context.Context, o wruntime.SaveDialogOptions) (string, error) {
	args := dialogArgs(o.Title, "", o.Filters)
	if name := filepath.Join(o.DefaultDirectory, o.DefaultFilename); name != "" {
		args = append(args, "--filename="+name)
	}
	return zenity(ctx, append(args, "--save", "--confirm-overwrite")...)
}
