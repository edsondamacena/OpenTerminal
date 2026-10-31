//go:build !linux

package webui

import "io/fs"

// Run só existe no Linux; nos outros sistemas o app usa a janela do Wails.
func Run(fs.FS, string) (bool, error) { return false, nil }
