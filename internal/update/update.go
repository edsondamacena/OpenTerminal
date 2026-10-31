// Package update procura versão nova nas Releases do GitHub, baixa o instalador da plataforma e
// confere o SHA-256 contra o SHA256SUMS da mesma Release antes de abrir. Sem SHA256SUMS ou com
// hash diferente, não instala. Só baixa de github.com e *.githubusercontent.com.
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Repo é "dono/nome" do repositório no GitHub (troca com -ldflags "-X openterminal/internal/update.Repo=...").
var Repo = "edsondamacena/OpenTerminal"

const (
	api        = "https://api.github.com/repos/%s/releases/latest"
	sumsName   = "SHA256SUMS"
	maxAsset   = 400 << 20 // instalador
	maxSums    = 64 << 10
	maxRelease = 1 << 20
)

var (
	ErrNoRepo     = errors.New("atualização sem repositório configurado")
	ErrNoAsset    = errors.New("a versão nova não tem instalador para este sistema")
	ErrNoChecksum = errors.New("a versão nova não tem SHA256SUMS: instalação recusada")
	ErrChecksum   = errors.New("o arquivo baixado não confere com o SHA256SUMS: instalação recusada")
	ErrHost       = errors.New("endereço de download fora do GitHub")
)

// Info é o que a tela mostra.
type Info struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	Notes     string `json:"notes"`
	PageURL   string `json:"pageUrl"`
	AssetName string `json:"assetName"`

	assetURL string
	sumsURL  string
}

type release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Body    string `json:"body"`
	Draft   bool   `json:"draft"`
	Pre     bool   `json:"prerelease"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// Check consulta a última Release. goos/goarch escolhem o instalador (.deb, .exe ou .apk).
func Check(ctx context.Context, client *http.Client, current, goos, goarch string) (Info, error) {
	info := Info{Current: strings.TrimPrefix(current, "v")}
	if Repo == "" {
		return info, ErrNoRepo
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(api, Repo), nil)
	if err != nil {
		return info, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return info, fmt.Errorf("GitHub respondeu %d", resp.StatusCode)
	}
	var r release
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxRelease)).Decode(&r); err != nil {
		return info, err
	}
	if r.Draft || r.Pre {
		return info, nil
	}
	info.Latest, info.Notes, info.PageURL = strings.TrimPrefix(r.TagName, "v"), r.Body, r.HTMLURL
	info.Available = Newer(info.Latest, info.Current)
	for _, a := range r.Assets {
		switch {
		case a.Name == sumsName:
			info.sumsURL = a.URL
		case info.assetURL == "" && Matches(a.Name, goos, goarch):
			info.AssetName, info.assetURL = a.Name, a.URL
		}
	}
	return info, nil
}

// Matches diz se o arquivo da Release é o instalador desta plataforma.
func Matches(name, goos, goarch string) bool {
	n := strings.ToLower(name)
	switch goos {
	case "linux":
		return strings.HasSuffix(n, "_"+goarch+".deb")
	case "windows":
		return strings.HasSuffix(n, ".exe") && strings.Contains(n, "installer")
	case "android":
		return strings.HasSuffix(n, ".apk")
	}
	return false
}

// Newer compara versões "maior.menor.correção" (sufixos como -rc são ignorados).
func Newer(latest, current string) bool {
	a, b := parts(latest), parts(current)
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

func parts(v string) [3]int {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	for i, p := range strings.SplitN(v, ".", 3) {
		out[i], _ = strconv.Atoi(p)
	}
	return out
}

// Client é o http.Client da atualização: não segue redirecionamento para fora do GitHub.
func Client() *http.Client {
	return &http.Client{Timeout: 10 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !allowed(req.URL.String()) {
			return ErrHost
		}
		return nil
	}}
}

func allowed(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" {
		return false
	}
	h := strings.ToLower(u.Hostname())
	return h == "github.com" || strings.HasSuffix(h, ".githubusercontent.com")
}

// Download baixa o instalador para dir e confere o SHA-256. Devolve o caminho do arquivo.
func Download(ctx context.Context, client *http.Client, info Info, dir string) (string, error) {
	if !info.Available || info.assetURL == "" {
		return "", ErrNoAsset
	}
	if info.sumsURL == "" {
		return "", ErrNoChecksum
	}
	if !allowed(info.assetURL) || !allowed(info.sumsURL) {
		return "", ErrHost
	}
	sums, err := get(ctx, client, info.sumsURL, maxSums)
	if err != nil {
		return "", err
	}
	want := ""
	for _, line := range strings.Split(string(sums), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == info.AssetName {
			want = strings.ToLower(f[0])
		}
	}
	if len(want) != 64 {
		return "", ErrNoChecksum
	}
	data, err := get(ctx, client, info.assetURL, maxAsset)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != want {
		return "", ErrChecksum
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, filepath.Base(info.AssetName))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func get(ctx context.Context, client *http.Client, raw string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if !allowed(resp.Request.URL.String()) { // redirecionou para fora do GitHub
		return nil, ErrHost
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download respondeu %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("arquivo de atualização grande demais")
	}
	return data, nil
}
