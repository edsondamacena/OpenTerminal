//go:build android || !(linux || windows)

package update

import "errors"

func Open(path string) error {
	if Installer == nil {
		return errors.ErrUnsupported
	}
	return Installer(path)
}
