package webui

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// O backend escuta só em 127.0.0.1, com porta aleatória, e o cofre fica atrás dele. Três travas:
//   - Host tem de ser exatamente 127.0.0.1:porta (um site com DNS apontado para 127.0.0.1, o
//     "DNS rebinding", chega com o Host dele e é recusado);
//   - o WebSocket só aceita a Origin da própria página;
//   - a primeira mensagem do WebSocket é o token aleatório desta execução, que só o navegador
//     aberto pelo app conhece (vai no fragmento da URL, que não sai em requisição nenhuma).
const (
	authTimeout  = 5 * time.Second
	maxMessage   = 16 << 20
	sendQueueLen = 4096
)

type Server struct {
	token    string
	addr     string
	origin   string
	listener net.Listener
	http     *http.Server
	assets   http.Handler
	dispatch *dispatcher
	errorFmt func(error) any
	upgrader websocket.Upgrader

	mu        sync.Mutex
	conns     map[*conn]struct{}
	connected bool
}

type conn struct {
	ws   *websocket.Conn
	send chan []byte
	once sync.Once
}

func (c *conn) close() { c.once.Do(func() { close(c.send) }) }

type inbound struct {
	Token string            `json:"t"`
	ID    uint64            `json:"id"`
	Name  string            `json:"m"`
	Args  []json.RawMessage `json:"a"`
}

type reply struct {
	ID     uint64 `json:"id"`
	Result any    `json:"r"` // sem omitempty: false, 0 e "" são respostas
	Error  any    `json:"e,omitempty"`
}

type event struct {
	Name    string `json:"ev"`
	Payload any    `json:"p"`
}

// NewServer abre a porta. assets é o frontend compilado (frontend/dist); bound são as structs que
// o Wails ligaria; errorFmt converte o erro como o ErrorFormatter do Wails.
func NewServer(assets fs.FS, bound []any, errorFmt func(error) any) (*Server, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		listener.Close()
		return nil, err
	}
	s := &Server{
		token:    hex.EncodeToString(raw),
		addr:     listener.Addr().String(),
		listener: listener,
		assets:   http.FileServerFS(assets),
		dispatch: newDispatcher(bound),
		errorFmt: errorFmt,
		conns:    map[*conn]struct{}{},
	}
	s.origin = "http://" + s.addr
	s.upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return r.Header.Get("Origin") == s.origin }}
	s.http = &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second}
	return s, nil
}

// URL é o endereço que o navegador abre; o token vai no fragmento.
func (s *Server) URL() string { return s.origin + "/#t=" + s.token }

func (s *Server) Serve() error {
	err := s.http.Serve(s.listener)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (s *Server) Close() error {
	s.mu.Lock()
	for c := range s.conns {
		delete(s.conns, c)
		c.close()
	}
	s.mu.Unlock()
	return s.http.Close()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Host != s.addr {
		http.Error(w, "host recusado", http.StatusForbidden)
		return
	}
	if r.URL.Path == "/ws" {
		s.serveWS(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Content-Security-Policy", "frame-ancestors 'none'")
	s.assets.ServeHTTP(w, r)
}

func (s *Server) serveWS(w http.ResponseWriter, r *http.Request) {
	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	ws.SetReadLimit(maxMessage)
	if !s.authenticate(ws) {
		_ = ws.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "token"), time.Now().Add(time.Second))
		ws.Close()
		return
	}
	c := &conn{ws: ws, send: make(chan []byte, sendQueueLen)}
	s.mu.Lock()
	s.conns[c] = struct{}{}
	s.connected = true
	s.mu.Unlock()
	go s.writeLoop(c)
	s.enqueue(c, map[string]bool{"ready": true})
	s.readLoop(c)
	s.mu.Lock()
	delete(s.conns, c)
	s.mu.Unlock()
	c.close()
}

func (s *Server) authenticate(ws *websocket.Conn) bool {
	_ = ws.SetReadDeadline(time.Now().Add(authTimeout))
	var msg inbound
	if err := ws.ReadJSON(&msg); err != nil {
		return false
	}
	_ = ws.SetReadDeadline(time.Time{})
	return subtle.ConstantTimeCompare([]byte(msg.Token), []byte(s.token)) == 1
}

func (s *Server) readLoop(c *conn) {
	for {
		var msg inbound
		if err := c.ws.ReadJSON(&msg); err != nil {
			return
		}
		// Cada chamada na sua goroutine, como no Wails: uma conexão SSH demorada não trava as outras.
		go func() {
			result, err := s.dispatch.call(msg.Name, msg.Args)
			out := reply{ID: msg.ID, Result: result}
			if err != nil {
				out = reply{ID: msg.ID, Error: s.errorFmt(err)}
			}
			s.enqueue(c, out)
		}()
	}
}

func (s *Server) writeLoop(c *conn) {
	defer c.ws.Close()
	for data := range c.send {
		if err := c.ws.WriteMessage(websocket.TextMessage, data); err != nil {
			return
		}
	}
}

// enqueue nunca bloqueia: um cliente que não lê (aba congelada) perde a conexão em vez de travar
// o PTY que está emitindo.
func (s *Server) enqueue(c *conn, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.conns[c]; !ok {
		return
	}
	select {
	case c.send <- data:
	default:
		delete(s.conns, c)
		c.close()
	}
}

// Emit manda o evento a todas as páginas autenticadas.
func (s *Server) Emit(name string, payload any) {
	data, err := json.Marshal(event{Name: name, Payload: payload})
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.conns {
		select {
		case c.send <- data:
		default:
			delete(s.conns, c)
			c.close()
		}
	}
}

// Connected diz se há página aberta.
func (s *Server) Connected() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.conns)
}

func (s *Server) everConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connected
}
