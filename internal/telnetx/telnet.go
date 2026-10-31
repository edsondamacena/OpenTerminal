// Package telnetx é o transporte telnet (RFC 854) das sessões: OLT, switch e roteador antigos que
// não falam SSH. Negocia só eco, supressão de go-ahead, tamanho da janela (NAWS) e tipo de
// terminal; recusa o resto. Com usuário e senha, responde ao login uma vez e sai do caminho.
package telnetx

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	iac  = 255
	dont = 254
	do   = 253
	wont = 252
	will = 251
	sb   = 250
	se   = 240

	optEcho  = 1
	optSGA   = 3
	optTType = 24
	optNAWS  = 31

	ttypeIs   = 0
	ttypeSend = 1
)

type Options struct {
	Address string
	Port    int
	Timeout time.Duration
	Term    string
	// Username e Password vazios: sem login automático, o usuário digita.
	Username string
	Password string
}

type Conn struct {
	conn  net.Conn
	term  string
	login *autoLogin

	wmu        sync.Mutex
	naws       bool
	cols, rows int

	done      chan struct{}
	err       error
	closeOnce sync.Once
}

func Dial(ctx context.Context, o Options) (*Conn, error) {
	d := net.Dialer{Timeout: o.Timeout}
	c, err := d.DialContext(ctx, "tcp", net.JoinHostPort(o.Address, strconv.Itoa(o.Port)))
	if err != nil {
		return nil, err
	}
	term := o.Term
	if term == "" {
		term = "xterm-256color"
	}
	t := &Conn{conn: c, term: term, done: make(chan struct{})}
	if o.Password != "" {
		t.login = newAutoLogin(o.Username, o.Password)
	}
	return t, nil
}

// Start começa a ler; o tamanho vale para quando o servidor pedir NAWS.
func (t *Conn) Start(cols, rows int, out io.Writer) error {
	t.wmu.Lock()
	t.cols, t.rows = cols, rows
	t.wmu.Unlock()
	go t.readLoop(out)
	return nil
}

func (t *Conn) Wait() error {
	<-t.done
	return t.err
}

// Write manda o que o usuário digitou: 0xFF dobrado e o Enter (CR) como CR NUL.
func (t *Conn) Write(p []byte) error {
	return t.send(encode(p))
}

func (t *Conn) Resize(cols, rows int) error {
	t.wmu.Lock()
	t.cols, t.rows = cols, rows
	naws := t.naws
	t.wmu.Unlock()
	if !naws {
		return nil
	}
	return t.send(nawsFrame(cols, rows))
}

func (t *Conn) Close() error {
	t.closeOnce.Do(func() { _ = t.conn.Close() })
	return nil
}

func (t *Conn) send(b []byte) error {
	t.wmu.Lock()
	defer t.wmu.Unlock()
	_, err := t.conn.Write(b)
	return err
}

func encode(p []byte) []byte {
	out := make([]byte, 0, len(p)+4)
	for i, b := range p {
		switch {
		case b == iac:
			out = append(out, iac, iac)
		case b == '\r' && (i+1 >= len(p) || p[i+1] != '\n'):
			out = append(out, '\r', 0)
		default:
			out = append(out, b)
		}
	}
	return out
}

func nawsFrame(cols, rows int) []byte {
	clamp := func(v int) int { return max(0, min(v, 0xFFFF)) }
	frame := []byte{iac, sb, optNAWS}
	for _, v := range []int{clamp(cols), clamp(rows)} {
		for _, b := range []byte{byte(v >> 8), byte(v)} {
			frame = append(frame, b)
			if b == iac {
				frame = append(frame, iac)
			}
		}
	}
	return append(frame, iac, se)
}

func (t *Conn) readLoop(out io.Writer) {
	defer close(t.done)
	r := &reader{conn: t}
	buf := make([]byte, 32*1024)
	for {
		n, err := t.conn.Read(buf)
		if n > 0 {
			data := r.feed(buf[:n])
			if len(data) > 0 {
				if _, werr := out.Write(data); werr != nil {
					t.err = werr
					return
				}
				if t.login != nil {
					if reply := t.login.see(data); reply != nil {
						_ = t.send(reply)
					}
				}
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				t.err = err
			}
			return
		}
	}
}

// reader separa os dados dos comandos IAC, mantendo o estado entre leituras.
type reader struct {
	conn  *Conn
	state int
	verb  byte
	sub   []byte
}

const (
	stData = iota
	stIAC
	stOpt
	stSub
	stSubIAC
)

func (r *reader) feed(in []byte) []byte {
	out := make([]byte, 0, len(in))
	for _, b := range in {
		switch r.state {
		case stData:
			if b == iac {
				r.state = stIAC
			} else {
				out = append(out, b)
			}
		case stIAC:
			switch b {
			case iac:
				out = append(out, iac)
				r.state = stData
			case do, dont, will, wont:
				r.verb, r.state = b, stOpt
			case sb:
				r.sub, r.state = r.sub[:0], stSub
			default: // NOP, GA e afins
				r.state = stData
			}
		case stOpt:
			r.conn.negotiate(r.verb, b)
			r.state = stData
		case stSub:
			if b == iac {
				r.state = stSubIAC
			} else if len(r.sub) < 512 {
				r.sub = append(r.sub, b)
			}
		case stSubIAC:
			switch b {
			case se:
				r.conn.subnegotiate(r.sub)
				r.state = stData
			case iac:
				if len(r.sub) < 512 {
					r.sub = append(r.sub, iac)
				}
				r.state = stSub
			default:
				r.state = stData
			}
		}
	}
	return out
}

func (t *Conn) negotiate(verb, opt byte) {
	switch verb {
	case do:
		switch opt {
		case optNAWS:
			t.wmu.Lock()
			already := t.naws
			t.naws = true
			cols, rows := t.cols, t.rows
			t.wmu.Unlock()
			if !already {
				_ = t.send([]byte{iac, will, optNAWS})
			}
			_ = t.send(nawsFrame(cols, rows))
		case optTType, optSGA:
			_ = t.send([]byte{iac, will, opt})
		default:
			_ = t.send([]byte{iac, wont, opt})
		}
	case will:
		if opt == optEcho || opt == optSGA {
			_ = t.send([]byte{iac, do, opt})
		} else {
			_ = t.send([]byte{iac, dont, opt})
		}
	case dont:
		if opt == optNAWS {
			t.wmu.Lock()
			t.naws = false
			t.wmu.Unlock()
		}
	}
	// WONT não pede resposta: confirmar de novo cria laço com servidores que repetem.
}

func (t *Conn) subnegotiate(sub []byte) {
	if len(sub) >= 2 && sub[0] == optTType && sub[1] == ttypeSend {
		frame := append([]byte{iac, sb, optTType, ttypeIs}, t.term...)
		_ = t.send(append(frame, iac, se))
	}
}
