// Package gdrive faz o login Google (OAuth com PKCE, retorno no 127.0.0.1) e guarda um arquivo
// na pasta oculta do app no Drive (appDataFolder). Só pede drive.appdata: o app não vê nenhum
// outro arquivo do usuário.
package gdrive

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Preenchidos no build (-ldflags -X) a partir de ~/.config/OpenTerminal/google-oauth.json, que
// fica fora do git. Para app de computador o Google não trata a chave do cliente como sigilosa.
var (
	ClientID     string
	ClientSecret string
)

const (
	authURL   = "https://accounts.google.com/o/oauth2/v2/auth"
	tokenURL  = "https://oauth2.googleapis.com/token"
	revokeURL = "https://oauth2.googleapis.com/revoke"
	scopes    = "openid email https://www.googleapis.com/auth/drive.appdata"
)

var ErrNotConfigured = errors.New("o login Google não foi configurado neste build")

// Client são as credenciais do cliente OAuth.
type Client struct{ ID, Secret string }

// LoadClient usa o que veio no build ou, sem isso, o JSON baixado do Google Cloud.
func LoadClient() (Client, error) {
	if ClientID != "" && ClientSecret != "" {
		return Client{ClientID, ClientSecret}, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return Client{}, ErrNotConfigured
	}
	raw, err := os.ReadFile(filepath.Join(dir, "OpenTerminal", "google-oauth.json"))
	if err != nil {
		return Client{}, ErrNotConfigured
	}
	var f struct {
		Installed struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
		} `json:"installed"`
	}
	if json.Unmarshal(raw, &f) != nil || f.Installed.ClientID == "" || f.Installed.ClientSecret == "" {
		return Client{}, ErrNotConfigured
	}
	return Client{f.Installed.ClientID, f.Installed.ClientSecret}, nil
}

// Grant é o resultado do login: o refresh token é o que se guarda (cifrado).
type Grant struct {
	RefreshToken string
	Email        string
}

// Login abre a página do Google com openURL e espera o retorno no 127.0.0.1.
func Login(ctx context.Context, c Client, openURL func(string)) (Grant, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return Grant{}, err
	}
	defer ln.Close()
	redirect := fmt.Sprintf("http://%s/callback", ln.Addr())
	verifier, state := randomString(), randomString()
	sum := sha256.Sum256([]byte(verifier))

	q := url.Values{
		"client_id":             {c.ID},
		"redirect_uri":          {redirect},
		"response_type":         {"code"},
		"scope":                 {scopes},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(sum[:])},
		"code_challenge_method": {"S256"},
		"state":                 {state},
		"access_type":           {"offline"},
		// Sem consent o Google não devolve refresh token num segundo login.
		"prompt": {"consent select_account"},
	}

	type result struct {
		code string
		err  error
	}
	done := make(chan result, 1)
	srv := &http.Server{ReadHeaderTimeout: 10 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}
		v := r.URL.Query()
		var res result
		switch {
		case v.Get("state") != state:
			res.err = errors.New("resposta do Google não confere (state)")
		case v.Get("error") != "":
			res.err = fmt.Errorf("o Google recusou o login: %s", v.Get("error"))
		case v.Get("code") == "":
			res.err = errors.New("o Google não devolveu o código")
		default:
			res.code = v.Get("code")
		}
		msg := "Pronto. Pode fechar esta aba e voltar ao OpenTerminal."
		if res.err != nil {
			msg = "Não deu certo: " + res.err.Error()
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width">`+
			`<title>OpenTerminal</title><body style="font:16px system-ui;padding:32px">%s</body>`, html.EscapeString(msg))
		select {
		case done <- res:
		default:
		}
	})}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	openURL(authURL + "?" + q.Encode())
	var res result
	select {
	case <-ctx.Done():
		return Grant{}, errors.New("o login Google não foi concluído a tempo")
	case res = <-done:
	}
	if res.err != nil {
		return Grant{}, res.err
	}
	tok, err := postToken(ctx, url.Values{
		"code":          {res.code},
		"client_id":     {c.ID},
		"client_secret": {c.Secret},
		"redirect_uri":  {redirect},
		"grant_type":    {"authorization_code"},
		"code_verifier": {verifier},
	})
	if err != nil {
		return Grant{}, err
	}
	if tok.RefreshToken == "" {
		return Grant{}, errors.New("o Google não devolveu acesso permanente; tente conectar de novo")
	}
	if !strings.Contains(tok.Scope, "drive.appdata") {
		return Grant{}, errors.New("é preciso marcar a permissão do Google Drive na tela do Google")
	}
	return Grant{RefreshToken: tok.RefreshToken, Email: emailOf(tok.IDToken)}, nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	IDToken      string `json:"id_token"`
	Error        string `json:"error"`
	Description  string `json:"error_description"`
}

// ErrRevoked: o acesso foi revogado ou expirou (no modo teste do Google, a cada 7 dias).
var ErrRevoked = errors.New("o acesso ao Google expirou ou foi removido; conecte de novo")

func postToken(ctx context.Context, form url.Values) (tokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return tokenResponse{}, err
	}
	defer resp.Body.Close()
	var tok tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return tokenResponse{}, fmt.Errorf("resposta inválida do Google (%s)", resp.Status)
	}
	if tok.Error == "invalid_grant" {
		return tokenResponse{}, ErrRevoked
	}
	if resp.StatusCode != http.StatusOK || tok.AccessToken == "" {
		return tokenResponse{}, fmt.Errorf("Google: %s %s", tok.Error, tok.Description)
	}
	return tok, nil
}

// Revoke desfaz o acesso no Google (ao desconectar). Erro aqui não impede apagar o token local.
func Revoke(ctx context.Context, refreshToken string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, revokeURL,
		strings.NewReader(url.Values{"token": {refreshToken}}.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// emailOf lê o e-mail do id_token só para mostrar na tela; ele veio direto do Google por TLS.
func emailOf(idToken string) string {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return ""
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Email string `json:"email"`
	}
	_ = json.Unmarshal(raw, &claims)
	return claims.Email
}

func randomString() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
