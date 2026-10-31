//go:build !windows

package sshx

import (
	"errors"
	"io"
	"net"
	"os"
)

// AgentOpener abre a conexão com um ssh-agent.
type AgentOpener func() (io.ReadWriteCloser, error)

func openSystemAgent() (io.ReadWriteCloser, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, errors.New("SSH_AUTH_SOCK ausente")
	}
	return net.Dial("unix", sock)
}
