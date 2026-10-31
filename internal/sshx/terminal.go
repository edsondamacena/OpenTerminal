package sshx

import (
	"io"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// Terminal é o shell remoto de uma sessão (implementa session.Conn).
type Terminal struct {
	client *Client

	mu    sync.Mutex
	sess  *ssh.Session
	stdin io.WriteCloser
}

func NewTerminal(c *Client) *Terminal {
	return &Terminal{client: c}
}

// Latency expõe à sessão o RTT medido pelo keepalive para a barra de status.
func (t *Terminal) Latency() time.Duration {
	return t.client.Latency()
}

// Start pede o PTY já com o tamanho do primeiro fit e abre o shell.
func (t *Terminal) Start(cols, rows int, out io.Writer) error {
	sess, err := t.client.NewSession()
	if err != nil {
		return err
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		_ = sess.Close()
		return err
	}
	sess.Stdout, sess.Stderr = out, out
	modes := ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400}
	if err := sess.RequestPty(t.client.term, rows, cols, modes); err != nil {
		_ = sess.Close()
		return err
	}
	if err := sess.Shell(); err != nil {
		_ = sess.Close()
		return err
	}
	t.mu.Lock()
	t.sess, t.stdin = sess, stdin
	t.mu.Unlock()
	return nil
}

// Wait espera o shell terminar ou a conexão cair; toda a saída já foi entregue ao voltar.
func (t *Terminal) Wait() error {
	t.mu.Lock()
	sess := t.sess
	t.mu.Unlock()
	if sess != nil {
		_ = sess.Wait()
	}
	return nil
}

func (t *Terminal) Write(p []byte) error {
	t.mu.Lock()
	stdin := t.stdin
	t.mu.Unlock()
	if stdin == nil {
		return nil
	}
	_, err := stdin.Write(p)
	return err
}

func (t *Terminal) Resize(cols, rows int) error {
	t.mu.Lock()
	sess := t.sess
	t.mu.Unlock()
	if sess == nil {
		return nil
	}
	return sess.WindowChange(rows, cols)
}

func (t *Terminal) Close() error {
	t.mu.Lock()
	sess := t.sess
	t.mu.Unlock()
	if sess != nil {
		_ = sess.Close()
	}
	return t.client.Close()
}
