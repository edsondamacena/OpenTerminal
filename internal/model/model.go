// Package model tem as entidades do cofre (data-model.md da 001 e da 002). As tags JSON seguem
// os tipos de frontend/src/shared/model/types.ts; campos opcionais são ponteiros (null).
package model

type AuthMethod string

const (
	AuthPassword AuthMethod = "password"
	AuthKey      AuthMethod = "key"
	AuthAsk      AuthMethod = "ask"
	AuthAgent    AuthMethod = "agent"
)

type Group struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	ParentID  *string `json:"parentId"`
	SortOrder int     `json:"sortOrder"`
}

type Host struct {
	ID               string     `json:"id"`
	Label            string     `json:"label"`
	Address          string     `json:"address"`
	Port             int        `json:"port"`
	Username         string     `json:"username"`
	AuthMethod       AuthMethod `json:"authMethod"`
	CredentialID     *string    `json:"credentialId"`
	GroupID          *string    `json:"groupId"`
	LegacyAlgorithms bool       `json:"legacyAlgorithms"`
	// Protocol vazio é SSH: hosts gravados antes do telnet não têm o campo.
	Protocol Protocol `json:"protocol,omitempty"`
	// DeviceIcon é o ícone de equipamento escolhido; vazio usa o padrão do protocolo.
	DeviceIcon string  `json:"deviceIcon,omitempty"`
	ProfileID  *string `json:"profileId"`
	CreatedAt  string  `json:"createdAt"`
	UpdatedAt  string  `json:"updatedAt"`
}

type Protocol string

const (
	ProtocolSSH    Protocol = "ssh"
	ProtocolTelnet Protocol = "telnet"
)

// IsTelnet diz se o host abre por telnet; qualquer outro valor (inclusive vazio) é SSH.
func (h Host) IsTelnet() bool { return h.Protocol == ProtocolTelnet }

type CredentialKind string

const (
	CredentialPassword CredentialKind = "password"
	CredentialKey      CredentialKind = "key"
)

// Credential guarda os dois tipos; só os campos do tipo correspondente são usados.
// Nunca sai pela ponte: a interface só recebe CredentialSummary.
type Credential struct {
	ID        string         `json:"id"`
	Kind      CredentialKind `json:"kind"`
	Name      string         `json:"name"`
	CreatedAt string         `json:"createdAt"`

	Password string `json:"password,omitempty"`

	PrivateKey  string  `json:"privateKey,omitempty"`
	Passphrase  *string `json:"passphrase,omitempty"`
	KeyType     string  `json:"keyType,omitempty"`
	PublicKey   string  `json:"publicKey,omitempty"`
	Fingerprint string  `json:"fingerprint,omitempty"`
	Origin      string  `json:"origin,omitempty"`
}

type KnownHost struct {
	ID          string `json:"id"`
	Endpoint    string `json:"endpoint"`
	KeyType     string `json:"keyType"`
	Fingerprint string `json:"fingerprint"`
	AcceptedAt  string `json:"acceptedAt"`
}

type ForwardRule struct {
	ID               string  `json:"id"`
	HostID           string  `json:"hostId"`
	Label            *string `json:"label"`
	BindAddress      string  `json:"bindAddress"`
	AllowNonLoopback bool    `json:"allowNonLoopback"`
	BindPort         int     `json:"bindPort"`
	DestHost         string  `json:"destHost"`
	DestPort         int     `json:"destPort"`
}

type VaultSettings struct {
	// 1–120; nil = desativado; padrão 15.
	AutoLockMinutes *int `json:"autoLockMinutes"`
	// 5–120; padrão 15.
	ConnectTimeoutSeconds int `json:"connectTimeoutSeconds"`
	// 10–300; padrão 30 (FR-120).
	KeepaliveSeconds int `json:"keepaliveSeconds"`
	// Padrão aplicado a hosts novos que omitirem legacyAlgorithms.
	LegacyDefault bool `json:"legacyDefault"`
	// Valor de TERM enviado ao servidor no pedido de PTY.
	Term string `json:"term"`
}

const DefaultTerm = "xterm-256color"

func ValidTerm(term string) bool {
	switch term {
	case "xterm-256color", "xterm", "vt100", "linux":
		return true
	default:
		return false
	}
}

type CursorStyle string

const (
	CursorBlock     CursorStyle = "block"
	CursorBar       CursorStyle = "bar"
	CursorUnderline CursorStyle = "underline"
)

type SchemeOrigin string

const (
	SchemeBuiltin  SchemeOrigin = "builtin"
	SchemeImported SchemeOrigin = "imported"
	SchemeCustom   SchemeOrigin = "custom"
)

type TerminalProfile struct {
	ID                string      `json:"id"`
	Name              string      `json:"name"`
	IsGlobal          bool        `json:"isGlobal"`
	FontFamily        string      `json:"fontFamily"`
	FontSize          float64     `json:"fontSize"`
	FontWeight        int         `json:"fontWeight"`
	FontWeightBold    int         `json:"fontWeightBold"`
	LineHeight        float64     `json:"lineHeight"`
	LetterSpacing     float64     `json:"letterSpacing"`
	Ligatures         bool        `json:"ligatures"`
	CursorStyle       CursorStyle `json:"cursorStyle"`
	CursorBlink       bool        `json:"cursorBlink"`
	CursorColor       *string     `json:"cursorColor"`
	Scrollback        int         `json:"scrollback"`
	BackgroundOpacity float64     `json:"backgroundOpacity"`
	Blur              int         `json:"blur"`
	Padding           int         `json:"padding"`
	ColorSchemeID     string      `json:"colorSchemeId"`
}

type ColorScheme struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Origin     SchemeOrigin `json:"origin"`
	Foreground string       `json:"foreground"`
	Background string       `json:"background"`
	Cursor     string       `json:"cursor"`
	Selection  string       `json:"selection"`
	ANSI       []string     `json:"ansi"`
}

func DefaultVaultSettings() VaultSettings {
	return VaultSettings{
		AutoLockMinutes: nil, ConnectTimeoutSeconds: 15, KeepaliveSeconds: 30,
		LegacyDefault: false, Term: DefaultTerm,
	}
}

func NormalizeVaultSettings(settings VaultSettings) VaultSettings {
	defaults := DefaultVaultSettings()
	if settings.ConnectTimeoutSeconds < 5 || settings.ConnectTimeoutSeconds > 120 {
		settings.ConnectTimeoutSeconds = defaults.ConnectTimeoutSeconds
	}
	if settings.KeepaliveSeconds < 10 || settings.KeepaliveSeconds > 300 {
		settings.KeepaliveSeconds = defaults.KeepaliveSeconds
	}
	if !ValidTerm(settings.Term) {
		settings.Term = defaults.Term
	}
	return settings
}

// Content é o cofre decifrado em memória (decifrado inteiro no desbloqueio, data-model.md).
type Content struct {
	Settings     VaultSettings
	Groups       []Group
	Hosts        []Host
	Credentials  []Credential
	KnownHosts   []KnownHost
	ForwardRules []ForwardRule
}

func EmptyContent() *Content {
	return &Content{
		Settings:     DefaultVaultSettings(),
		Groups:       []Group{},
		Hosts:        []Host{},
		Credentials:  []Credential{},
		KnownHosts:   []KnownHost{},
		ForwardRules: []ForwardRule{},
	}
}

// Clone devolve uma cópia profunda (as mutações trabalham sobre um rascunho).
func (c *Content) Clone() *Content {
	out := &Content{
		Settings:     c.Settings,
		Groups:       make([]Group, len(c.Groups)),
		Hosts:        make([]Host, len(c.Hosts)),
		Credentials:  make([]Credential, len(c.Credentials)),
		KnownHosts:   append([]KnownHost{}, c.KnownHosts...),
		ForwardRules: make([]ForwardRule, len(c.ForwardRules)),
	}
	out.Settings.AutoLockMinutes = clonePtr(c.Settings.AutoLockMinutes)
	for i, g := range c.Groups {
		g.ParentID = clonePtr(g.ParentID)
		out.Groups[i] = g
	}
	for i, h := range c.Hosts {
		h.CredentialID = clonePtr(h.CredentialID)
		h.GroupID = clonePtr(h.GroupID)
		h.ProfileID = clonePtr(h.ProfileID)
		out.Hosts[i] = h
	}
	for i, cr := range c.Credentials {
		cr.Passphrase = clonePtr(cr.Passphrase)
		out.Credentials[i] = cr
	}
	for i, r := range c.ForwardRules {
		r.Label = clonePtr(r.Label)
		out.ForwardRules[i] = r
	}
	return out
}

func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// Ptr é um atalho para campos opcionais.
func Ptr[T any](v T) *T { return &v }
