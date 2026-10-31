package domain

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/ssh"

	"openterminal/internal/apperr"
	"openterminal/internal/model"
	"openterminal/internal/validate"
)

const MaxKeyBytes = 64 * 1024

var supportedKeyTypes = map[string]bool{
	ssh.KeyAlgoED25519:  true,
	ssh.KeyAlgoECDSA256: true,
	ssh.KeyAlgoECDSA384: true,
	ssh.KeyAlgoECDSA521: true,
	ssh.KeyAlgoRSA:      true,
}

// CredentialSummary é a única forma de credencial que sai pela ponte: sem segredos.
type CredentialSummary struct {
	ID              string               `json:"id"`
	Name            string               `json:"name"`
	Kind            model.CredentialKind `json:"kind"`
	CreatedAt       string               `json:"createdAt"`
	KeyType         *string              `json:"keyType"`
	PublicKey       *string              `json:"publicKey"`
	Fingerprint     *string              `json:"fingerprint"`
	Origin          *string              `json:"origin"`
	UsedByHostCount int                  `json:"usedByHostCount"`
}

type ParsedKey struct {
	KeyType     string
	PublicKey   string
	Fingerprint string
	// Passphrase de fato necessária (nil se a chave não é cifrada).
	Passphrase *string
}

func keyErr(code apperr.Code) error { return apperr.New(code) }

// ParsePrivateKey valida a chave; nada é salvo se falhar.
func ParsePrivateKey(text string, passphrase *string) (ParsedKey, error) {
	if len(text) > MaxKeyBytes {
		return ParsedKey{}, keyErr(apperr.KeyUnsupported)
	}
	var used *string
	raw, err := ssh.ParseRawPrivateKey([]byte(text))
	var missing *ssh.PassphraseMissingError
	if errors.As(err, &missing) {
		if passphrase == nil || *passphrase == "" {
			return ParsedKey{}, keyErr(apperr.KeyBadPassphrase)
		}
		raw, err = ssh.ParseRawPrivateKeyWithPassphrase([]byte(text), []byte(*passphrase))
		if errors.Is(err, x509.IncorrectPasswordError) {
			return ParsedKey{}, keyErr(apperr.KeyBadPassphrase)
		}
		used = passphrase
	}
	if err != nil {
		return ParsedKey{}, keyErr(apperr.KeyUnsupported)
	}
	signer, err := ssh.NewSignerFromKey(raw)
	if err != nil {
		return ParsedKey{}, keyErr(apperr.KeyUnsupported)
	}
	pub := signer.PublicKey()
	if !supportedKeyTypes[pub.Type()] {
		return ParsedKey{}, keyErr(apperr.KeyUnsupported)
	}
	return ParsedKey{
		KeyType:     pub.Type(),
		PublicKey:   strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub))),
		Fingerprint: Fingerprint(pub),
		Passphrase:  used,
	}, nil
}

func findCredential(c *model.Content, id string) *model.Credential {
	for i := range c.Credentials {
		if c.Credentials[i].ID == id {
			return &c.Credentials[i]
		}
	}
	return nil
}

func FindCredential(c *model.Content, id string) (model.Credential, error) {
	cr := findCredential(c, id)
	if cr == nil {
		return model.Credential{}, notFound()
	}
	return *cr, nil
}

func validSecret(s string) bool {
	n := utf8.RuneCountInString(s)
	return n >= 1 && n <= 1024
}

func CreatePassword(c *model.Content, name, password, now string) (model.Credential, error) {
	clean, err := validate.Text(name, 1, 100)
	if err != nil || !validSecret(password) {
		return model.Credential{}, invalid()
	}
	cr := model.Credential{Kind: model.CredentialPassword, ID: NewID(), Name: clean, Password: password, CreatedAt: now}
	c.Credentials = append(c.Credentials, cr)
	return cr, nil
}

func addKey(c *model.Content, name, privateKey string, passphrase *string, origin, now string) (model.Credential, error) {
	clean, err := validate.Text(name, 1, 100)
	if err != nil {
		return model.Credential{}, err
	}
	p, err := ParsePrivateKey(privateKey, passphrase)
	if err != nil {
		return model.Credential{}, err
	}
	cr := model.Credential{Kind: model.CredentialKey, ID: NewID(), Name: clean, PrivateKey: privateKey,
		Passphrase: p.Passphrase, KeyType: p.KeyType, PublicKey: p.PublicKey,
		Fingerprint: p.Fingerprint, Origin: origin, CreatedAt: now}
	c.Credentials = append(c.Credentials, cr)
	return cr, nil
}

func ImportKey(c *model.Content, name, pemText string, passphrase *string, now string) (model.Credential, error) {
	return addKey(c, name, pemText, passphrase, "imported", now)
}

// GenerateEd25519 gera uma chave no formato OpenSSH, cifrada se houver passphrase.
func GenerateEd25519(c *model.Content, name string, passphrase *string, now string) (model.Credential, error) {
	if passphrase != nil && !validSecret(*passphrase) {
		return model.Credential{}, invalid()
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return model.Credential{}, err
	}
	var block *pem.Block
	if passphrase != nil {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte(*passphrase))
	} else {
		block, err = ssh.MarshalPrivateKey(priv, "")
	}
	if err != nil {
		return model.Credential{}, err
	}
	return addKey(c, name, string(pem.EncodeToMemory(block)), passphrase, "generated", now)
}

func RenameCredential(c *model.Content, id, name string) (model.Credential, error) {
	clean, err := validate.Text(name, 1, 100)
	if err != nil {
		return model.Credential{}, err
	}
	cr := findCredential(c, id)
	if cr == nil {
		return model.Credential{}, notFound()
	}
	cr.Name = clean
	return *cr, nil
}

func UsedByHostCount(c *model.Content, id string) int {
	n := 0
	for _, h := range c.Hosts {
		if h.CredentialID != nil && *h.CredentialID == id {
			n++
		}
	}
	return n
}

func RemoveCredential(c *model.Content, id string) error {
	if findCredential(c, id) == nil {
		return notFound()
	}
	if UsedByHostCount(c, id) > 0 {
		return invalid()
	}
	out := c.Credentials[:0]
	for _, cr := range c.Credentials {
		if cr.ID != id {
			out = append(out, cr)
		}
	}
	c.Credentials = out
	return nil
}

func ToSummary(c *model.Content, cr model.Credential) CredentialSummary {
	s := CredentialSummary{ID: cr.ID, Name: cr.Name, Kind: cr.Kind, CreatedAt: cr.CreatedAt,
		UsedByHostCount: UsedByHostCount(c, cr.ID)}
	if cr.Kind == model.CredentialKey {
		s.KeyType, s.PublicKey, s.Fingerprint, s.Origin = &cr.KeyType, &cr.PublicKey, &cr.Fingerprint, &cr.Origin
	}
	return s
}
