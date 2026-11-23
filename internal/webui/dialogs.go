package webui

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Diálogos de arquivo pelo zenity (GTK, presente no Mint/Ubuntu/GNOME). Cancelar devolve "",
// como no Wails.

func zenity(ctx context.Context, args ...string) (string, error) {
	path, err := exec.LookPath("zenity")
	if err != nil {
		return "", errors.New("instale o zenity para escolher arquivos (sudo apt install zenity)")
	}
	// Sem janela-mãe o Cinnamon (prevenção de roubo de foco) abre o diálogo atrás do app.
	if id, ok := appWindowID(); ok {
		args = append(args, "--attach="+strconv.FormatUint(id, 10))
	}
	cmd := exec.CommandContext(ctx, path, append([]string{"--file-selection"}, args...)...)
	done := make(chan struct{})
	defer close(done)
	go raiseDialog(done)
	out, err := cmd.Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

// listWindows devolve as linhas do `wmctrl -lx` (id, mesa, classe, ...); sem wmctrl, nada.
func listWindows() [][]string {
	wmctrl, err := exec.LookPath("wmctrl")
	if err != nil {
		return nil
	}
	out, err := exec.Command(wmctrl, "-lx").Output()
	if err != nil {
		return nil
	}
	var rows [][]string
	for _, line := range strings.Split(string(out), "\n") {
		if f := strings.Fields(line); len(f) >= 3 {
			rows = append(rows, f)
		}
	}
	return rows
}

func appWindowID() (uint64, bool) {
	for _, f := range listWindows() {
		if strings.HasSuffix(f[2], ".OpenTerminal") {
			id, err := strconv.ParseUint(f[0], 0, 64)
			return id, err == nil
		}
	}
	return 0, false
}

// raiseDialog espera a janela do zenity aparecer e a põe na frente, acima do app.
func raiseDialog(done <-chan struct{}) {
	wmctrl, err := exec.LookPath("wmctrl")
	if err != nil {
		return
	}
	for i := 0; i < 40; i++ {
		select {
		case <-done:
			return
		case <-time.After(150 * time.Millisecond):
		}
		for _, f := range listWindows() {
			if strings.HasPrefix(strings.ToLower(f[2]), "zenity.") {
				_ = exec.Command(wmctrl, "-ir", f[0], "-b", "add,above").Run()
				_ = exec.Command(wmctrl, "-ia", f[0]).Run()
				return
			}
		}
	}
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
