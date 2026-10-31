// Package devicekey guarda a chave do cofre no cofre do sistema (Secret Service no Linux,
// Keystore no Android) para o desbloqueio automático. Sem cofre do sistema, nada é guardado.
package devicekey

// Keeper guarda um segredo por nome. Get devolve ok=false quando não há (ou não dá para ler).
type Keeper interface {
	Get(name string) ([]byte, bool)
	Set(name string, secret []byte) error
	Delete(name string) error
}
