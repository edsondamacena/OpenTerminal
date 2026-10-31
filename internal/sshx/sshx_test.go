package sshx

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/pem"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"
	"golang.org/x/crypto/ssh"

	"openterminal/internal/apperr"
	"openterminal/internal/domain"
	"openterminal/internal/model"
	"openterminal/internal/session"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		// go-winio mantém um processador IOCP global até o fim do processo.
		goleak.IgnoreAnyFunction("github.com/Microsoft/go-winio.ioCompletionProcessor"),
	)
}

// --- servidor SSH em processo ----------------------------------------------------------------

type ptyReq struct {
	term       string
	cols, rows uint32
}

type server struct {
	t       *testing.T
	ln      net.Listener
	hostKey ssh.Signer
	cfg     *ssh.ServerConfig

	// Comportamento.
	ignoreKeepalive bool
	authorized      ssh.PublicKey

	mu         sync.Mutex
	conns      []net.Conn
	ptys       []ptyReq
	resizes    []ptyReq
	keepalives int
	shellAfter bool // o shell chegou depois do pty-req
	wg         sync.WaitGroup
}

func newHostKey(t *testing.T) ssh.Signer {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	s, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func startServer(t *testing.T, mod func(*server)) *server {
	t.Helper()
	s := &server{t: t, hostKey: newHostKey(t)}
	s.cfg = &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pw []byte) (*ssh.Permissions, error) {
			if c.User() == "u" && string(pw) == "pw" {
				return nil, nil
			}
			return nil, io.EOF
		},
		PublicKeyCallback: func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if s.authorized != nil && bytes.Equal(key.Marshal(), s.authorized.Marshal()) {
				return nil, nil
			}
			return nil, io.EOF
		},
		KeyboardInteractiveCallback: func(c ssh.ConnMetadata, ch ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			a, err := ch("", "", []string{"Código:"}, []bool{true})
			if err == nil && len(a) == 1 && a[0] == "123" {
				return nil, nil
			}
			return nil, io.EOF
		},
	}
	if mod != nil {
		mod(s)
	}
	s.cfg.AddHostKey(s.hostKey)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s.ln = ln
	s.wg.Add(1)
	go s.accept()
	t.Cleanup(s.stop)
	return s
}

func (s *server) port() int { return s.ln.Addr().(*net.TCPAddr).Port }

func (s *server) stop() {
	_ = s.ln.Close()
	s.mu.Lock()
	for _, c := range s.conns {
		_ = c.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *server) accept() {
	defer s.wg.Done()
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		s.conns = append(s.conns, c)
		s.mu.Unlock()
		s.wg.Add(1)
		go s.serve(c)
	}
}

func (s *server) serve(c net.Conn) {
	defer s.wg.Done()
	sc, chans, reqs, err := ssh.NewServerConn(c, s.cfg)
	if err != nil {
		_ = c.Close()
		return
	}
	defer sc.Close()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for r := range reqs {
			if r.Type == "keepalive@openssh.com" {
				s.mu.Lock()
				s.keepalives++
				ignore := s.ignoreKeepalive
				s.mu.Unlock()
				if ignore {
					continue
				}
			}
			_ = r.Reply(false, nil)
		}
	}()
	for nc := range chans {
		ch, creqs, err := nc.Accept()
		if err != nil {
			continue
		}
		s.wg.Add(1)
		go s.session(ch, creqs)
	}
}

func (s *server) session(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer s.wg.Done()
	defer ch.Close()
	var hadPty bool
	for r := range reqs {
		switch r.Type {
		case "pty-req":
			// string term, uint32 cols, uint32 rows, ...
			n := binary.BigEndian.Uint32(r.Payload)
			term := string(r.Payload[4 : 4+n])
			p := r.Payload[4+n:]
			s.mu.Lock()
			s.ptys = append(s.ptys, ptyReq{term, binary.BigEndian.Uint32(p), binary.BigEndian.Uint32(p[4:])})
			s.mu.Unlock()
			hadPty = true
			_ = r.Reply(true, nil)
		case "window-change":
			s.mu.Lock()
			s.resizes = append(s.resizes, ptyReq{"", binary.BigEndian.Uint32(r.Payload), binary.BigEndian.Uint32(r.Payload[4:])})
			s.mu.Unlock()
		case "shell":
			s.mu.Lock()
			s.shellAfter = hadPty
			s.mu.Unlock()
			_ = r.Reply(true, nil)
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				_, _ = ch.Write([]byte("$ "))
				buf := make([]byte, 1024)
				for {
					n, err := ch.Read(buf)
					if n > 0 {
						if bytes.Contains(buf[:n], []byte("exit")) {
							_, _ = ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
							_ = ch.Close()
							return
						}
						_, _ = ch.Write(buf[:n])
					}
					if err != nil {
						return
					}
				}
			}()
		default:
			_ = r.Reply(false, nil)
		}
	}
}

// --- dublês ------------------------------------------------------------------------------------

type fakeKnown struct {
	mu      sync.Mutex
	content *model.Content
}

func newKnown() *fakeKnown { return &fakeKnown{content: model.EmptyContent()} }

func (k *fakeKnown) Check(ep, kt, fp string) (domain.HostKeyCheck, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return domain.CheckHostKey(k.content, ep, kt, fp), nil
}

func (k *fakeKnown) Known(ep string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	for _, h := range k.content.KnownHosts {
		if h.Endpoint == ep {
			return h.KeyType, nil
		}
	}
	return "", nil
}

func (k *fakeKnown) Accept(ep, kt, fp string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	domain.AcceptHostKey(k.content, ep, kt, fp, domain.Now())
	return nil
}

func (k *fakeKnown) count() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return len(k.content.KnownHosts)
}

type fakeIx struct {
	ctx       context.Context
	acceptKey bool
	answers   []string
	block     bool // espera o cancelamento em vez de responder

	mu       sync.Mutex
	states   []session.State
	hostKeys []session.HostKeyQuestion
	prompts  []session.PromptQuestion
}

func (f *fakeIx) SetState(s session.State) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.states = append(f.states, s)
}

func (f *fakeIx) AskHostKey(q session.HostKeyQuestion) (bool, error) {
	f.mu.Lock()
	f.hostKeys = append(f.hostKeys, q)
	f.mu.Unlock()
	if f.block {
		<-f.ctx.Done()
		return false, f.ctx.Err()
	}
	return f.acceptKey, nil
}

func (f *fakeIx) AskPrompt(q session.PromptQuestion) ([]string, error) {
	f.mu.Lock()
	f.prompts = append(f.prompts, q)
	f.mu.Unlock()
	return f.answers, nil
}

func opts(s *server, known *fakeKnown, auth Auth) Options {
	return Options{Target: Target{Address: "127.0.0.1", Port: s.port(), Username: "u"}, Auth: auth,
		ConnectTimeout: 5 * time.Second, KnownHosts: known}
}

func dial(t *testing.T, o Options, ix *fakeIx) (*Client, error) {
	t.Helper()
	if ix.ctx == nil {
		ix.ctx = context.Background()
	}
	c, err := Dial(ix.ctx, o, ix)
	if c != nil {
		t.Cleanup(func() { _ = c.Close() })
	}
	return c, err
}

func wantCode(t *testing.T, err error, code apperr.Code) {
	t.Helper()
	if got := apperr.CodeOf(err); got != code {
		t.Fatalf("erro %v (%s), esperado %s", err, got, code)
	}
}

// --- testes ------------------------------------------------------------------------------------

func TestPasswordAuthAndNewHostKey(t *testing.T) {
	s := startServer(t, nil)
	known := newKnown()
	ix := &fakeIx{acceptKey: true}
	if _, err := dial(t, opts(s, known, Auth{Method: model.AuthPassword, Password: "pw"}), ix); err != nil {
		t.Fatal(err)
	}
	if len(ix.hostKeys) != 1 || ix.hostKeys[0].Kind != "new" || ix.hostKeys[0].Fingerprint != domain.Fingerprint(s.hostKey.PublicKey()) {
		t.Fatalf("pergunta de host key %+v", ix.hostKeys)
	}
	if known.count() != 1 {
		t.Fatal("host key não foi gravada")
	}
	// Segunda conexão: chave conhecida, sem pergunta.
	ix2 := &fakeIx{}
	if _, err := dial(t, opts(s, known, Auth{Method: model.AuthPassword, Password: "pw"}), ix2); err != nil {
		t.Fatal(err)
	}
	if len(ix2.hostKeys) != 0 {
		t.Fatal("perguntou de novo por chave conhecida")
	}
}

func TestChangedHostKeyBlocksUntilAccepted(t *testing.T) {
	s := startServer(t, nil)
	known := newKnown()
	ep := domain.EndpointOf("127.0.0.1", s.port())
	_ = known.Accept(ep, "ssh-ed25519", "SHA256:antiga")
	ix := &fakeIx{acceptKey: false}
	_, err := dial(t, opts(s, known, Auth{Method: model.AuthPassword, Password: "pw"}), ix)
	wantCode(t, err, apperr.HostKeyRejected)
	if len(ix.hostKeys) != 1 || ix.hostKeys[0].Kind != "changed" || ix.hostKeys[0].PreviousFingerprint == nil || *ix.hostKeys[0].PreviousFingerprint != "SHA256:antiga" {
		t.Fatalf("pergunta %+v", ix.hostKeys)
	}
	if fp, _ := known.Check(ep, "ssh-ed25519", "SHA256:antiga"); fp.Kind != domain.HostKeyMatch {
		t.Fatal("chave antiga foi substituída sem aceite")
	}
}

func TestWrongPasswordFallsBackToPromptAndFails(t *testing.T) {
	s := startServer(t, nil)
	ix := &fakeIx{acceptKey: true, answers: []string{"errado"}}
	_, err := dial(t, opts(s, newKnown(), Auth{Method: model.AuthPassword, Password: "nao"}), ix)
	wantCode(t, err, apperr.AuthFailed)
	if len(ix.prompts) != 1 || ix.prompts[0].Prompts[0].Text != "Código:" || !ix.prompts[0].Prompts[0].Echo {
		t.Fatalf("prompts %+v", ix.prompts)
	}
}

func TestKeyboardInteractiveAnswered(t *testing.T) {
	s := startServer(t, func(s *server) { s.cfg.PasswordCallback = nil })
	ix := &fakeIx{acceptKey: true, answers: []string{"123"}}
	if _, err := dial(t, opts(s, newKnown(), Auth{Method: model.AuthAsk}), ix); err != nil {
		t.Fatal(err)
	}
	if ix.states[len(ix.states)-1] != session.StateAuthenticating {
		t.Fatalf("estados %v", ix.states)
	}
}

func TestPromptCancelledIsAuthFailed(t *testing.T) {
	s := startServer(t, func(s *server) { s.cfg.PasswordCallback = nil })
	ix := &fakeIx{acceptKey: true, answers: nil}
	_, err := dial(t, opts(s, newKnown(), Auth{Method: model.AuthAsk}), ix)
	wantCode(t, err, apperr.AuthFailed)
}

func TestAskPasswordOnlyServer(t *testing.T) {
	s := startServer(t, func(s *server) { s.cfg.KeyboardInteractiveCallback = nil })
	ix := &fakeIx{acceptKey: true, answers: []string{"pw"}}
	if _, err := dial(t, opts(s, newKnown(), Auth{Method: model.AuthAsk}), ix); err != nil {
		t.Fatal(err)
	}
	if len(ix.prompts) != 1 || ix.prompts[0].Prompts[0].Text != "Senha de u@127.0.0.1:" {
		t.Fatalf("prompts %+v", ix.prompts)
	}
}

func TestStoredPasswordAnswersKeyboardInteractive(t *testing.T) {
	s := startServer(t, func(s *server) {
		s.cfg.PasswordCallback = nil
		s.cfg.KeyboardInteractiveCallback = func(c ssh.ConnMetadata, ch ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			a, err := ch("", "", []string{"Password:"}, []bool{false})
			if err == nil && len(a) == 1 && a[0] == "pw" {
				return nil, nil
			}
			return nil, io.EOF
		}
	})
	ix := &fakeIx{acceptKey: true}
	if _, err := dial(t, opts(s, newKnown(), Auth{Method: model.AuthPassword, Password: "pw"}), ix); err != nil {
		t.Fatal(err)
	}
	if len(ix.prompts) != 0 {
		t.Fatal("perguntou a senha que já estava guardada")
	}
}

func encryptedKey(t *testing.T, passphrase string) (string, ssh.PublicKey) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	block, err := ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte(passphrase))
	if err != nil {
		t.Fatal(err)
	}
	signer, _ := ssh.NewSignerFromKey(priv)
	return string(pem.EncodeToMemory(block)), signer.PublicKey()
}

func TestKeyWithPassphrase(t *testing.T) {
	pemText, pub := encryptedKey(t, "frase")
	s := startServer(t, func(s *server) { s.authorized = pub })
	ix := &fakeIx{acceptKey: true}
	auth := Auth{Method: model.AuthKey, PrivateKey: pemText, Passphrase: model.Ptr("frase")}
	if _, err := dial(t, opts(s, newKnown(), auth), ix); err != nil {
		t.Fatal(err)
	}
	if len(ix.prompts) != 0 {
		t.Fatal("caiu no keyboard-interactive")
	}
}

func TestAgentUnavailable(t *testing.T) {
	s := startServer(t, nil)
	o := opts(s, newKnown(), Auth{Method: model.AuthAgent})
	o.Agent = func() (io.ReadWriteCloser, error) { return nil, io.ErrClosedPipe }
	_, err := dial(t, o, &fakeIx{acceptKey: true})
	wantCode(t, err, apperr.AgentUnavailable)
}

func legacyOnly(s *server) {
	s.cfg.KeyExchanges = []string{ssh.InsecureKeyExchangeDH14SHA1}
	s.cfg.Ciphers = []string{ssh.InsecureCipherAES128CBC}
}

func TestLegacyAlgorithmsOffByDefault(t *testing.T) {
	s := startServer(t, legacyOnly)
	_, err := dial(t, opts(s, newKnown(), Auth{Method: model.AuthPassword, Password: "pw"}), &fakeIx{acceptKey: true})
	wantCode(t, err, apperr.NoCommonAlgorithm)
}

func TestLegacyAlgorithmsWhenEnabled(t *testing.T) {
	s := startServer(t, legacyOnly)
	o := opts(s, newKnown(), Auth{Method: model.AuthPassword, Password: "pw"})
	o.LegacyAlgorithms = true
	if _, err := dial(t, o, &fakeIx{acceptKey: true}); err != nil {
		t.Fatal(err)
	}
}

func TestHostKeyAlgorithmsPreferKnownType(t *testing.T) {
	got := hostKeyAlgorithms(false, ssh.KeyAlgoRSA)
	if got[0] != ssh.KeyAlgoRSASHA512 || got[1] != ssh.KeyAlgoRSASHA256 {
		t.Fatalf("ordem %v", got[:3])
	}
	for _, a := range got {
		if a == ssh.KeyAlgoRSA {
			t.Fatal("ssh-rsa (SHA-1) sem algoritmos legados")
		}
	}
	if got := hostKeyAlgorithms(true, ""); got[len(got)-1] != ssh.KeyAlgoRSA {
		t.Fatal("ssh-rsa ausente com algoritmos legados")
	}
}

func TestCancelWhileAwaitingHostKey(t *testing.T) {
	s := startServer(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	ix := &fakeIx{ctx: ctx, block: true}
	done := make(chan error, 1)
	go func() {
		_, err := Dial(ctx, opts(s, newKnown(), Auth{Method: model.AuthPassword, Password: "pw"}), ix)
		done <- err
	}()
	deadline := time.Now().Add(2 * time.Second)
	for {
		ix.mu.Lock()
		n := len(ix.hostKeys)
		ix.mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("pergunta não chegou")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		wantCode(t, err, apperr.Cancelled)
	case <-time.After(time.Second):
		t.Fatal("Dial pendurou depois do cancelamento")
	}
}

func TestConnectRefused(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	o := Options{Target: Target{Address: "127.0.0.1", Port: port, Username: "u"},
		Auth: Auth{Method: model.AuthPassword, Password: "pw"}, ConnectTimeout: time.Second, KnownHosts: newKnown()}
	_, err := Dial(context.Background(), o, &fakeIx{})
	wantCode(t, err, apperr.ConnectRefused)
}

type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func TestTerminalPtyBeforeShellAndResize(t *testing.T) {
	s := startServer(t, nil)
	o := opts(s, newKnown(), Auth{Method: model.AuthPassword, Password: "pw"})
	o.Term = "linux"
	c, err := dial(t, o, &fakeIx{acceptKey: true})
	if err != nil {
		t.Fatal(err)
	}
	term := NewTerminal(c)
	var out lockedBuf
	if err := term.Start(120, 40, &out); err != nil {
		t.Fatal(err)
	}
	waited := make(chan struct{})
	go func() {
		_ = term.Wait()
		close(waited)
	}()
	_ = term.Write([]byte("oi"))
	_ = term.Resize(100, 30)
	deadline := time.Now().Add(2 * time.Second)
	for out.String() != "$ oi" {
		if time.Now().After(deadline) {
			t.Fatalf("saída %q", out.String())
		}
		time.Sleep(time.Millisecond)
	}
	_ = term.Write([]byte("exit"))
	select {
	case <-waited:
	case <-time.After(2 * time.Second):
		t.Fatal("Wait não voltou com o fim do shell")
	}
	_ = term.Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.ptys) != 1 || s.ptys[0] != (ptyReq{"linux", 120, 40}) || !s.shellAfter {
		t.Fatalf("pty %v shellDepois=%v", s.ptys, s.shellAfter)
	}
	if len(s.resizes) != 1 || s.resizes[0] != (ptyReq{"", 100, 30}) {
		t.Fatalf("resizes %v", s.resizes)
	}
}

func TestKeepaliveSentAndDeadServerDropped(t *testing.T) {
	s := startServer(t, nil)
	o := opts(s, newKnown(), Auth{Method: model.AuthPassword, Password: "pw"})
	o.Keepalive = 20 * time.Millisecond
	c, err := dial(t, o, &fakeIx{acceptKey: true})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	s.mu.Lock()
	n := s.keepalives
	s.ignoreKeepalive = true
	s.mu.Unlock()
	if n < 3 {
		t.Fatalf("keepalives = %d", n)
	}
	dropped := make(chan struct{})
	go func() {
		_ = c.Wait()
		close(dropped)
	}()
	select {
	case <-dropped:
	case <-time.After(2 * time.Second):
		t.Fatal("conexão sem resposta não caiu")
	}
}
