//go:build windows

package appwin

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const spiGetHighContrast = 0x0042

var systemParametersInfo = windows.NewLazySystemDLL("user32.dll").NewProc("SystemParametersInfoW")

type highContrast struct {
	Size        uint32
	Flags       uint32
	ColorScheme *uint16
}

type RegistryAppearanceReader struct{}

func (RegistryAppearanceReader) ReadAppearance() (SystemAppearance, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return SystemAppearance{}, err
	}
	defer key.Close()
	transparency, _, err := key.GetIntegerValue("EnableTransparency")
	if err != nil {
		return SystemAppearance{}, err
	}
	light, _, err := key.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return SystemAppearance{}, err
	}
	hc := highContrast{Size: uint32(unsafe.Sizeof(highContrast{}))}
	ok, _, callErr := systemParametersInfo.Call(spiGetHighContrast, uintptr(hc.Size), uintptr(unsafe.Pointer(&hc)), 0)
	if ok == 0 {
		return SystemAppearance{}, callErr
	}
	var accent *uint64
	dwm, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\DWM`, registry.QUERY_VALUE)
	if err == nil {
		defer dwm.Close()
		value, _, valueErr := dwm.GetIntegerValue("AccentColor")
		switch {
		case valueErr == nil:
			accent = &value
		case !errors.Is(valueErr, registry.ErrNotExist):
			return SystemAppearance{}, valueErr
		}
	} else if !errors.Is(err, registry.ErrNotExist) {
		return SystemAppearance{}, err
	}
	return appearanceFromValues(transparency, light, hc.Flags, accent), nil
}
