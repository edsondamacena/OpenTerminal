package gdrive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"sync"
	"time"

	"openterminal/internal/cloudsync"
)

const (
	fileName  = "openterminal-vault.sync"
	filesURL  = "https://www.googleapis.com/drive/v3/files"
	uploadURL = "https://www.googleapis.com/upload/drive/v3/files"
	maxFile   = 64 << 20
)

// Drive é o cloudsync.Remote no Google Drive. Renova o access token sozinho.
type Drive struct {
	client  Client
	refresh string
	http    *http.Client

	mu      sync.Mutex
	access  string
	expires time.Time
	fileID  string
}

func NewDrive(c Client, refreshToken string) *Drive {
	return &Drive{client: c, refresh: refreshToken, http: &http.Client{Timeout: 90 * time.Second}}
}

func (d *Drive) token(ctx context.Context) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.access != "" && time.Until(d.expires) > time.Minute {
		return d.access, nil
	}
	tok, err := postToken(ctx, url.Values{
		"client_id":     {d.client.ID},
		"client_secret": {d.client.Secret},
		"refresh_token": {d.refresh},
		"grant_type":    {"refresh_token"},
	})
	if err != nil {
		return "", err
	}
	d.access, d.expires = tok.AccessToken, time.Now().Add(time.Duration(tok.ExpiresIn)*time.Second)
	return d.access, nil
}

func (d *Drive) do(ctx context.Context, method, u string, body io.Reader, contentType string) (*http.Response, error) {
	tok, err := d.token(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := d.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		d.mu.Lock()
		d.access = ""
		d.mu.Unlock()
		return nil, errors.New("o Google recusou o acesso; tente de novo")
	}
	if resp.StatusCode/100 != 2 {
		defer resp.Body.Close()
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("Google Drive %s: %s", resp.Status, bytes.TrimSpace(msg))
	}
	return resp, nil
}

// find acha o arquivo do cofre; com mais de um (dois aparelhos criaram juntos), o mais recente.
func (d *Drive) find(ctx context.Context) (string, error) {
	d.mu.Lock()
	id := d.fileID
	d.mu.Unlock()
	if id != "" {
		return id, nil
	}
	q := url.Values{
		"spaces":  {"appDataFolder"},
		"q":       {fmt.Sprintf("name = '%s' and trashed = false", fileName)},
		"orderBy": {"modifiedTime desc"},
		"fields":  {"files(id)"},
	}
	resp, err := d.do(ctx, http.MethodGet, filesURL+"?"+q.Encode(), nil, "")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var list struct {
		Files []struct {
			ID string `json:"id"`
		} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return "", err
	}
	if len(list.Files) == 0 {
		return "", nil
	}
	d.mu.Lock()
	d.fileID = list.Files[0].ID
	d.mu.Unlock()
	return list.Files[0].ID, nil
}

func (d *Drive) Download(ctx context.Context) ([]byte, error) {
	id, err := d.find(ctx)
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, cloudsync.ErrNoFile
	}
	resp, err := d.do(ctx, http.MethodGet, filesURL+"/"+url.PathEscape(id)+"?alt=media", nil, "")
	if err != nil {
		// Arquivo apagado por fora: procura de novo na próxima vez.
		d.mu.Lock()
		d.fileID = ""
		d.mu.Unlock()
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFile+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxFile {
		return nil, errors.New("o arquivo da nuvem é grande demais")
	}
	return data, nil
}

func (d *Drive) Upload(ctx context.Context, data []byte) error {
	id, err := d.find(ctx)
	if err != nil {
		return err
	}
	if id != "" {
		resp, err := d.do(ctx, http.MethodPatch, uploadURL+"/"+url.PathEscape(id)+"?uploadType=media",
			bytes.NewReader(data), "application/octet-stream")
		if err != nil {
			return err
		}
		resp.Body.Close()
		return nil
	}
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	meta, _ := json.Marshal(map[string]any{"name": fileName, "parents": []string{"appDataFolder"}})
	part, _ := w.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/json; charset=UTF-8"}})
	_, _ = part.Write(meta)
	part, _ = w.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/octet-stream"}})
	_, _ = part.Write(data)
	_ = w.Close()
	resp, err := d.do(ctx, http.MethodPost, uploadURL+"?uploadType=multipart&fields=id", &body,
		"multipart/related; boundary="+w.Boundary())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err == nil && created.ID != "" {
		d.mu.Lock()
		d.fileID = created.ID
		d.mu.Unlock()
	}
	return nil
}
