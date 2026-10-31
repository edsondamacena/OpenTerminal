package gdrive

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// Só roda com OT_OAUTH_PROBE=1: confere na rede que o Google aceita o cliente e o redirect.
func TestProbeGoogleAcceptsClient(t *testing.T) {
	if os.Getenv("OT_OAUTH_PROBE") != "1" {
		t.Skip()
	}
	c, err := LoadClient()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	got := make(chan string, 1)
	go func() { _, _ = Login(ctx, c, func(u string) { got <- u }) }()
	u := <-got
	resp, err := http.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	page := string(body)
	for _, bad := range []string{"invalid_client", "redirect_uri_mismatch", "invalid_request", "Erro 400", "Error 400", "invalid_scope"} {
		if strings.Contains(page, bad) {
			t.Fatalf("Google recusou: %s (status %s, final %s)", bad, resp.Status, resp.Request.URL.Host+resp.Request.URL.Path)
		}
	}
	t.Logf("status %s, página final %s%s", resp.Status, resp.Request.URL.Host, resp.Request.URL.Path)
}
