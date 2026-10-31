//go:build windows

package sshx

import (
	"flag"
	"io"
	"os"

	"github.com/Microsoft/go-winio"
)

const windowsOpenSSHAgentPipe = `\\.\pipe\openssh-ssh-agent`

// AgentOpener abre a conexão com um ssh-agent.
type AgentOpener func() (io.ReadWriteCloser, error)

// systemAgentPipe mantém a superfície de produção presa ao agente OpenSSH do Windows. O
// override só existe dentro de binários de teste, para os testes de integração criarem um pipe.
func systemAgentPipe() string {
	if flag.Lookup("test.v") != nil {
		if pipe := os.Getenv("OPENTERMINAL_AGENT_PIPE"); pipe != "" {
			return pipe
		}
	}
	return windowsOpenSSHAgentPipe
}

func openSystemAgent() (io.ReadWriteCloser, error) {
	return winio.DialPipe(systemAgentPipe(), nil)
}
