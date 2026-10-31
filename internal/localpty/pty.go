// Package localpty abre shells locais num pseudo terminal (ConPTY no Windows) e implementa
// session.Conn.
package localpty

import (
	"io"
	"os"
	"sync"

	gopty "github.com/aymanbagabas/go-pty"
)

// PTY é o pseudo terminal usado pela sessão (go-pty atrás desta interface).
type PTY interface {
	io.ReadWriteCloser
	Resize(cols, rows int) error
}

// Process é um shell local preparado; só inicia no Start, com o primeiro tamanho válido.
type Process struct {
	path string
	args []string

	mu       sync.Mutex
	pty      PTY
	cmd      *gopty.Cmd
	out      io.Writer
	exitCode *int
	closeOne sync.Once
	exited   chan struct{}
}

func New(shell Shell) (*Process, error) {
	resolved, err := resolveShell(shell)
	if err != nil {
		return nil, err
	}
	return &Process{path: resolved.path, args: append([]string{}, resolved.args...), exited: make(chan struct{})}, nil
}

func (p *Process) Path() string { return p.path }

func (p *Process) Start(cols, rows int, out io.Writer) error {
	pt, err := gopty.New()
	if err != nil {
		return err
	}
	if err := pt.Resize(cols, rows); err != nil {
		_ = pt.Close()
		return err
	}
	cmd := pt.Command(p.path, p.args...)
	cmd.Env = append(os.Environ(), ptyEnv()...)
	if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}
	if err := cmd.Start(); err != nil {
		_ = pt.Close()
		return err
	}
	p.mu.Lock()
	p.pty, p.cmd, p.out = pt, cmd, out
	p.mu.Unlock()
	return nil
}

// Wait copia a saída até o shell terminar. No ConPTY a leitura só termina quando o pseudo
// console é fechado, então quem espera o processo fecha o PTY.
func (p *Process) Wait() error {
	p.mu.Lock()
	pt, cmd, out := p.pty, p.cmd, p.out
	p.mu.Unlock()
	if cmd == nil {
		return nil
	}
	go func() {
		_ = cmd.Wait()
		if cmd.ProcessState != nil {
			code := cmd.ProcessState.ExitCode()
			p.mu.Lock()
			p.exitCode = &code
			p.mu.Unlock()
		}
		p.closePty()
		close(p.exited)
	}()
	_, _ = io.Copy(out, pt)
	<-p.exited
	return nil
}

func (p *Process) ExitCode() *int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exitCode
}

func (p *Process) Write(b []byte) error {
	p.mu.Lock()
	pt := p.pty
	p.mu.Unlock()
	if pt == nil {
		return nil
	}
	_, err := pt.Write(b)
	return err
}

func (p *Process) Resize(cols, rows int) error {
	p.mu.Lock()
	pt := p.pty
	p.mu.Unlock()
	if pt == nil {
		return nil
	}
	return pt.Resize(cols, rows)
}

// Close encerra o shell (e o que ele iniciou no console) e libera o PTY.
func (p *Process) Close() error {
	p.mu.Lock()
	cmd := p.cmd
	p.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	p.closePty()
	return nil
}

func (p *Process) closePty() {
	p.closeOne.Do(func() {
		p.mu.Lock()
		pt := p.pty
		p.mu.Unlock()
		if pt != nil {
			_ = pt.Close()
		}
	})
}
