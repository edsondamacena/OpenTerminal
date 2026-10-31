// Núcleo do OpenTerminal no Android. O app Java (android/) executa este binário, lê a URL da
// interface e abre num WebView. Diálogos de arquivo e links externos vão para o Java por linhas
// JSON no stdout; as respostas voltam pelo stdin. Nada de dado do usuário passa por aqui além de
// caminhos de arquivo escolhidos por ele.
package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"openterminal/frontend"
	"openterminal/internal/bridge"
	"openterminal/internal/webui"
)

// request vai para o Java; reply volta dele.
type request struct {
	Op    string `json:"op"`
	ID    int    `json:"id,omitempty"`
	URL   string `json:"url,omitempty"`
	Title string `json:"title,omitempty"`
	Name  string `json:"name,omitempty"`
	Path  string `json:"path,omitempty"`
	Data  string `json:"data,omitempty"`
}

type reply struct {
	ID   int    `json:"id"`
	Path string `json:"path"`
}

type host struct {
	mu      sync.Mutex
	out     *json.Encoder
	next    int
	pending map[int]chan string
}

func (h *host) send(r request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	_ = h.out.Encode(r)
}

// ask manda o pedido e espera o caminho; "" é cancelamento.
func (h *host) ask(ctx context.Context, r request) (string, error) {
	h.mu.Lock()
	h.next++
	r.ID = h.next
	ch := make(chan string, 1)
	h.pending[r.ID] = ch
	_ = h.out.Encode(r)
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.pending, r.ID)
		h.mu.Unlock()
	}()
	select {
	case p := <-ch:
		return p, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (h *host) readReplies() {
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 64*1024), 64*1024)
	for sc.Scan() {
		var r reply
		if json.Unmarshal(sc.Bytes(), &r) != nil {
			continue
		}
		h.mu.Lock()
		ch := h.pending[r.ID]
		h.mu.Unlock()
		if ch != nil {
			ch <- r.Path
		}
	}
}

// waitWritten avisa o Java quando o arquivo de um "salvar" parou de crescer: ele copia para o
// destino escolhido e apaga a cópia temporária.
func (h *host) waitWritten(path string) {
	deadline := time.Now().Add(10 * time.Minute)
	var last int64 = -1
	for time.Now().Before(deadline) {
		time.Sleep(700 * time.Millisecond)
		st, err := os.Stat(path)
		if err != nil {
			continue
		}
		if st.Size() == last && st.Size() > 0 {
			h.send(request{Op: "saved", Path: path})
			return
		}
		last = st.Size()
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "uso: openterminal-android <pasta de dados>")
		os.Exit(2)
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "OpenTerminal não pôde iniciar:", err)
		os.Exit(1)
	}
}

func run(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	// Sem HOME no Android: bibliotecas que procuram a pasta do usuário caem na pasta de dados.
	_ = os.Setenv("HOME", dataDir)
	dist, err := fs.Sub(frontend.Assets, "dist")
	if err != nil {
		return err
	}
	h := &host{out: json.NewEncoder(os.Stdout), pending: map[int]chan string{}}
	go h.readReplies()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var server *webui.Server
	emit := func(_ context.Context, name string, payload any) {
		if server != nil {
			server.Emit(name, payload)
		}
	}
	open := func(ctx context.Context, o wruntime.OpenDialogOptions) (string, error) {
		return h.ask(ctx, request{Op: "open", Title: o.Title})
	}
	bridge.SetShell(bridge.Shell{
		Emit:             emit,
		Quit:             func(context.Context) { cancel() },
		Minimise:         func(context.Context) {},
		ToggleMaximise:   func(context.Context) {},
		OpenURL:          func(_ context.Context, url string) { h.send(request{Op: "openurl", URL: url}) },
		ClipboardSetText: func(_ context.Context, text string) error { emit(nil, "app:clipboard", text); return nil },
		OpenFile:         open,
		OpenFiles: func(ctx context.Context, o wruntime.OpenDialogOptions) ([]string, error) {
			p, err := open(ctx, o)
			if p == "" || err != nil {
				return nil, err
			}
			return []string{p}, nil
		},
		OpenDirectory: func(context.Context, wruntime.OpenDialogOptions) (string, error) {
			return "", errors.New("escolher pasta ainda não funciona no Android")
		},
		SaveFile: func(ctx context.Context, o wruntime.SaveDialogOptions) (string, error) {
			p, err := h.ask(ctx, request{Op: "save", Title: o.Title, Name: filepath.Base(o.DefaultFilename)})
			if p != "" && err == nil {
				go h.waitWritten(p)
			}
			return p, err
		},
	})
	bridge.SetDeviceKeeper(keystore{h})
	services, err := bridge.NewServices(dataDir)
	if err != nil {
		return err
	}
	server, err = webui.NewServer(dist, services.Bound(), bridge.FormatError)
	if err != nil {
		return err
	}
	go func() {
		if err := server.Serve(); err != nil {
			fmt.Fprintln(os.Stderr, "servidor local:", err)
			cancel()
		}
	}()
	services.Startup(ctx)
	defer func() {
		services.Shutdown(ctx)
		_ = server.Close()
	}()
	h.send(request{Op: "url", URL: server.URL()})

	// O Java fecha o stdin ou manda SIGTERM quando o app sai.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	select {
	case <-ctx.Done():
	case <-sig:
	}
	return nil
}

// keystore guarda a chave do desbloqueio automático no Android Keystore, pelo app Java: o Go
// manda o segredo em base64 pela entrada padrão e o Java cifra com uma chave que não sai do
// aparelho.
type keystore struct{ h *host }

func (k keystore) call(r request) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return k.h.ask(ctx, r)
}

func (k keystore) Get(name string) ([]byte, bool) {
	p, err := k.call(request{Op: "keyget", Name: name})
	if err != nil || p == "" {
		return nil, false
	}
	b, err := base64.StdEncoding.DecodeString(p)
	return b, err == nil && len(b) > 0
}

func (k keystore) Set(name string, secret []byte) error {
	p, err := k.call(request{Op: "keyset", Name: name, Data: base64.StdEncoding.EncodeToString(secret)})
	if err != nil {
		return err
	}
	if p != "ok" {
		return errors.New("o Keystore do Android recusou")
	}
	return nil
}

func (k keystore) Delete(name string) error {
	_, err := k.call(request{Op: "keydel", Name: name})
	return err
}
