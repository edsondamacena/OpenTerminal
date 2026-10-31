//go:build android || !(linux || windows)

package update

import "errors"

// Open: sem instalador automático aqui (Android instala pela página da Release ou pela Play).
func Open(string) error { return errors.ErrUnsupported }
