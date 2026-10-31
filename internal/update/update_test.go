package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rt responde como o GitHub, sem rede.
type rt map[string]string

func (m rt) RoundTrip(r *http.Request) (*http.Response, error) {
	body, ok := m[r.URL.String()]
	code := 200
	if !ok {
		code, body = 404, ""
	}
	return &http.Response{StatusCode: code, Body: ioNop(body), Request: r, Header: http.Header{}}, nil
}

type nopCloser struct{ *strings.Reader }

func (nopCloser) Close() error { return nil }
func ioNop(s string) nopCloser { return nopCloser{strings.NewReader(s)} }

const dl = "https://github.com/dono/OpenTerminal/releases/download/v2.1.0/"

func fake(deb string, sums string) *http.Client {
	sumsAsset := `,{"name":"SHA256SUMS","browser_download_url":"` + dl + `SHA256SUMS"}`
	if sums == "" { // Release publicada sem o SHA256SUMS
		sumsAsset = ""
	}
	m := rt{
		fmt.Sprintf(api, "dono/OpenTerminal"): `{"tag_name":"v2.1.0","html_url":"https://github.com/dono/OpenTerminal/releases/tag/v2.1.0",
			"body":"novidades","assets":[{"name":"openterminal_2.1.0_amd64.deb","browser_download_url":"` + dl + `openterminal_2.1.0_amd64.deb"},
			{"name":"OpenTerminal-amd64-installer.exe","browser_download_url":"` + dl + `OpenTerminal-amd64-installer.exe"}` + sumsAsset + `]}`,
		dl + "openterminal_2.1.0_amd64.deb": deb,
	}
	if sums != "" {
		m[dl+"SHA256SUMS"] = sums
	}
	return &http.Client{Transport: m}
}

func hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

func TestCheckEscolheOInstaladorDaPlataforma(t *testing.T) {
	Repo = "dono/OpenTerminal"
	defer func() { Repo = "edsondamacena/OpenTerminal" }()
	i, err := Check(context.Background(), fake("x", ""), "2.0.0", "linux", "amd64")
	if err != nil || !i.Available || i.Latest != "2.1.0" || i.AssetName != "openterminal_2.1.0_amd64.deb" {
		t.Fatalf("%+v %v", i, err)
	}
	i, _ = Check(context.Background(), fake("x", ""), "2.0.0", "windows", "amd64")
	if i.AssetName != "OpenTerminal-amd64-installer.exe" {
		t.Fatalf("windows: %+v", i)
	}
	i, _ = Check(context.Background(), fake("x", ""), "v2.1.0", "linux", "amd64")
	if i.Available {
		t.Fatal("mesma versão não é atualização")
	}
}

func TestDownloadConfereOSHA256(t *testing.T) {
	Repo = "dono/OpenTerminal"
	defer func() { Repo = "edsondamacena/OpenTerminal" }()
	ctx, dir := context.Background(), t.TempDir()
	deb := "conteudo do pacote"

	c := fake(deb, hash(deb)+"  openterminal_2.1.0_amd64.deb\n")
	i, _ := Check(ctx, c, "2.0.0", "linux", "amd64")
	p, err := Download(ctx, c, i, dir)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != deb || filepath.Dir(p) != dir {
		t.Fatalf("arquivo errado: %s", p)
	}

	c = fake(deb+"adulterado", hash(deb)+"  openterminal_2.1.0_amd64.deb\n")
	i, _ = Check(ctx, c, "2.0.0", "linux", "amd64")
	if _, err := Download(ctx, c, i, t.TempDir()); !errors.Is(err, ErrChecksum) {
		t.Fatalf("hash diferente devia recusar, veio %v", err)
	}

	c = fake(deb, "")
	i, _ = Check(ctx, c, "2.0.0", "linux", "amd64")
	if _, err := Download(ctx, c, i, t.TempDir()); !errors.Is(err, ErrNoChecksum) {
		t.Fatalf("sem SHA256SUMS devia recusar, veio %v", err)
	}
}

func TestSoBaixaDoGitHub(t *testing.T) {
	i := Info{Available: true, AssetName: "a.deb", assetURL: "https://evil.example/a.deb", sumsURL: dl + "SHA256SUMS"}
	if _, err := Download(context.Background(), &http.Client{Transport: rt{}}, i, t.TempDir()); !errors.Is(err, ErrHost) {
		t.Fatalf("host de fora devia recusar, veio %v", err)
	}
	if !allowed("https://objects.githubusercontent.com/x") || allowed("http://github.com/x") || allowed("https://github.com.evil.io/x") {
		t.Fatal("allowed errado")
	}
}

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{{"2.1.0", "2.0.0", true}, {"2.0.10", "2.0.9", true}, {"2.0.0", "2.0.0", false}, {"1.9.9", "2.0.0", false}, {"v3.0.0-rc.1", "2.9.0", true}} {
		if got := Newer(c.a, c.b); got != c.want {
			t.Fatalf("Newer(%s,%s)=%v", c.a, c.b, got)
		}
	}
}
