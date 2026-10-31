//go:build !linux || android

package devicekey

// Default: sem cofre do sistema conhecido aqui (o Android registra o dele no main).
func Default() Keeper { return nil }
