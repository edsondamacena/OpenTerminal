package webui

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gorilla/websocket"
)

type fakeService struct{}

func (fakeService) Echo(s string) (string, error) { return s, nil }
func (fakeService) False() (bool, error)          { return false, nil }
func (fakeService) Fail() error                   { return errors.New("falhou") }
func (fakeService) Panic() (int, error)           { panic("x") }

func startServer(t *testing.T) *Server {
	t.Helper()
	assets := fstest.MapFS{"index.html": {Data: []byte("<html>ok</html>")}}
	s, err := NewServer(assets, []any{&fakeService{}}, func(err error) any { return "E:" + err.Error() })
	if err != nil {
		t.Fatal(err)
	}
	go s.Serve()
	t.Cleanup(func() { s.Close() })
	return s
}

func token(s *Server) string { return s.token }

func dial(t *testing.T, s *Server, origin string) (*websocket.Conn, error) {
	t.Helper()
	h := http.Header{}
	if origin != "" {
		h.Set("Origin", origin)
	}
	ws, _, err := websocket.DefaultDialer.Dial("ws://"+s.addr+"/ws", h)
	return ws, err
}

func TestRejectsForeignHost(t *testing.T) {
	s := startServer(t)
	req, _ := http.NewRequest("GET", "http://"+s.addr+"/", nil)
	req.Host = "evil.example:" + strings.Split(s.addr, ":")[1]
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("host estranho: %d", resp.StatusCode)
	}
	resp, err = http.Get("http://" + s.addr + "/")
	if err != nil || resp.StatusCode != 200 || resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Fatalf("host certo: %v %v", resp, err)
	}
	resp.Body.Close()
}

func TestRejectsForeignOrigin(t *testing.T) {
	s := startServer(t)
	for _, origin := range []string{"", "http://evil.example", "http://localhost:" + strings.Split(s.addr, ":")[1]} {
		if ws, err := dial(t, s, origin); err == nil {
			ws.Close()
			t.Fatalf("origin %q aceita", origin)
		}
	}
}

func TestRejectsBadToken(t *testing.T) {
	s := startServer(t)
	for _, tok := range []string{"", "00", strings.Repeat("0", 64)} {
		ws, err := dial(t, s, s.origin)
		if err != nil {
			t.Fatal(err)
		}
		_ = ws.WriteJSON(map[string]string{"t": tok})
		// Nem "ready" nem resposta: a conexão é fechada.
		_ = ws.WriteJSON(map[string]any{"id": 1, "m": "fakeService.Echo", "a": []string{`"x"`}})
		_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
		if _, msg, err := ws.ReadMessage(); err == nil {
			t.Fatalf("token %q: recebeu %s", tok, msg)
		}
		ws.Close()
	}
	if s.everConnected() {
		t.Fatal("conexão sem token contou como página")
	}
}

func TestCallsAndEvents(t *testing.T) {
	s := startServer(t)
	ws, err := dial(t, s, s.origin)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	_ = ws.WriteJSON(map[string]string{"t": token(s)})
	read := func() map[string]any {
		var m map[string]any
		_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
		if err := ws.ReadJSON(&m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	if m := read(); m["ready"] != true {
		t.Fatalf("esperava ready: %v", m)
	}
	cases := []struct {
		name string
		args []any
		key  string
		want any
	}{
		{"fakeService.Echo", []any{"olá"}, "r", "olá"},
		{"fakeService.False", nil, "r", false},
		{"fakeService.Fail", nil, "e", "E:falhou"},
		{"fakeService.Panic", nil, "e", "E:pânico no método fakeService.Panic"},
		{"fakeService.Nope", nil, "e", "E:método desconhecido: fakeService.Nope"},
	}
	for i, c := range cases {
		_ = ws.WriteJSON(map[string]any{"id": i + 1, "m": c.name, "a": c.args})
		m := read()
		got, ok := m[c.key]
		if !ok || got != c.want {
			t.Fatalf("%s: %v", c.name, m)
		}
	}
	s.Emit("term:data", "abc")
	if m := read(); m["ev"] != "term:data" || m["p"] != "abc" {
		t.Fatalf("evento: %v", m)
	}
}

func TestURLKeepsTokenInFragment(t *testing.T) {
	s := startServer(t)
	u, err := url.Parse(s.URL())
	if err != nil || u.Host != s.addr || u.RawQuery != "" || u.Fragment != "t="+token(s) || len(token(s)) != 64 {
		t.Fatalf("URL: %s", s.URL())
	}
}
