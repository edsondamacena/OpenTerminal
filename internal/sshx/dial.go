// Package sshx conecta e autentica no servidor SSH (x/crypto/ssh) e abre o terminal remoto.
package sshx

import (
	"context"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"

	"openterminal/internal/apperr"
	"openterminal/internal/domain"
	"openterminal/internal/model"
	"openterminal/internal/session"
)

type Target struct {
	Address  string
	Port     int
	Username string
}

type Auth struct {
	Method     model.AuthMethod
	Password   string
	PrivateKey string
	Passphrase *string
}

// KnownHosts dá acesso às host keys aceitas (no cofre).
type KnownHosts interface {
	Check(endpoint, keyType, fingerprint string) (domain.HostKeyCheck, error)
	// Known devolve o tipo da chave já aceita para o endpoint ("" se não houver).
	Known(endpoint string) (string, error)
	Accept(endpoint, keyType, fingerprint string) error
}

type Options struct {
	Target
	Auth Auth
	// ConnectTimeout vale para a rede; o tempo esperando o usuário não conta.
	ConnectTimeout   time.Duration
	Keepalive        time.Duration
	LegacyAlgorithms bool
	Term             string
	KnownHosts       KnownHosts
	// Agent abre a conexão com o ssh-agent; nil usa o do sistema.
	Agent AgentOpener
}

// Client é a conexão autenticada; Close também para o keepalive.
type Client struct {
	*ssh.Client
	stop         chan struct{}
	term         string
	once         sync.Once
	latencyNanos atomic.Int64
}

func (c *Client) Close() error {
	c.once.Do(func() { close(c.stop) })
	return c.Client.Close()
}

// Latency devolve o RTT da última resposta de keepalive, ou zero antes da primeira resposta.
func (c *Client) Latency() time.Duration {
	return time.Duration(c.latencyNanos.Load())
}

// deadline limita o tempo de rede da conexão e pausa enquanto o usuário responde.
type deadline struct {
	conn net.Conn
	d    time.Duration
}

func (x deadline) pause()  { _ = x.conn.SetDeadline(time.Time{}) }
func (x deadline) resume() { _ = x.conn.SetDeadline(time.Now().Add(x.d)) }

// Dial conecta, verifica a host key (TOFU) e autentica. Desiste quando ctx é cancelado.
func Dial(ctx context.Context, o Options, ix session.Interactor) (*Client, error) {
	if o.ConnectTimeout <= 0 {
		o.ConnectTimeout = 15 * time.Second
	}
	if !model.ValidTerm(o.Term) {
		o.Term = model.DefaultTerm
	}
	endpoint := domain.EndpointOf(o.Address, o.Port)
	known, err := o.KnownHosts.Known(endpoint)
	if err != nil {
		return nil, err
	}
	a := &authState{o: o, ix: ix}
	methods, cleanup, err := a.methods()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	d := net.Dialer{Timeout: o.ConnectTimeout}
	addr := net.JoinHostPort(o.Address, strconv.Itoa(o.Port))
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	dl := deadline{conn, o.ConnectTimeout}
	a.dl = dl
	dl.resume()

	hk := &hostKeyCheck{o: o, ix: ix, dl: dl, endpoint: endpoint}
	cfg := &ssh.ClientConfig{
		User:              o.Username,
		Auth:              methods,
		HostKeyCallback:   hk.callback,
		HostKeyAlgorithms: hostKeyAlgorithms(o.LegacyAlgorithms, known),
	}
	cfg.KeyExchanges, cfg.Ciphers, cfg.MACs = transportAlgorithms(o.LegacyAlgorithms)

	c, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		_ = conn.Close()
		switch {
		case ctx.Err() != nil:
			return nil, ctx.Err()
		case hk.failure != nil:
			return nil, hk.failure
		case a.failure != nil:
			return nil, a.failure
		}
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	client := &Client{Client: ssh.NewClient(c, chans, reqs), stop: make(chan struct{}), term: o.Term}
	if ctx.Err() != nil {
		_ = client.Close()
		return nil, ctx.Err()
	}
	if o.Keepalive > 0 {
		go func() {
			_ = runKeepalive(client.stop, client.Client, o.Keepalive, func(rtt time.Duration) {
				client.latencyNanos.Store(int64(rtt))
			})
		}()
	}
	return client, nil
}

type hostKeyCheck struct {
	o        Options
	ix       session.Interactor
	dl       deadline
	endpoint string
	failure  error
}

func (h *hostKeyCheck) callback(_ string, _ net.Addr, key ssh.PublicKey) error {
	keyType, fp := key.Type(), domain.Fingerprint(key)
	chk, err := h.o.KnownHosts.Check(h.endpoint, keyType, fp)
	if err != nil {
		h.failure = err
		return err
	}
	if chk.Kind != domain.HostKeyMatch {
		q := session.HostKeyQuestion{Kind: string(chk.Kind), KeyType: keyType, Fingerprint: fp}
		if chk.Previous != nil {
			q.PreviousFingerprint = &chk.Previous.Fingerprint
		}
		h.dl.pause()
		ok, err := h.ix.AskHostKey(q)
		h.dl.resume()
		if err == nil && !ok {
			err = apperr.New(apperr.HostKeyRejected)
		}
		if err == nil {
			err = h.o.KnownHosts.Accept(h.endpoint, keyType, fp)
		}
		if err != nil {
			h.failure = err
			return err
		}
	}
	h.ix.SetState(session.StateAuthenticating)
	return nil
}
