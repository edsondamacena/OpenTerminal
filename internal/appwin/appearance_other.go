//go:build !windows

package appwin

// RegistryAppearanceReader fora do Windows não tem registro para consultar: usa tema escuro,
// sem transparência e sem cor de destaque do sistema.
type RegistryAppearanceReader struct{}

func (RegistryAppearanceReader) ReadAppearance() (SystemAppearance, error) {
	return appearanceFromValues(0, 0, 0, nil), nil
}
