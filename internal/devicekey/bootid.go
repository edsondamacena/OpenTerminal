package devicekey

import (
	"os"
	"strings"
)

// BootID identifica a inicialização atual do sistema (muda a cada vez que o PC liga ou reinicia).
// Vazio quando o sistema não informa (aí o desbloqueio automático não depende do boot).
func BootID() string {
	b, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
