package domain

import (
	"encoding/json"
	"net"
	"regexp"
	"strings"
	"unicode/utf8"

	"openterminal/internal/model"
	"openterminal/internal/validate"
)

var (
	hostnameRe = regexp.MustCompile(`(?i)^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`)
	// Sem espaços nem caracteres de controle.
	usernameRe = regexp.MustCompile(`^[^\s\p{Z}\p{Cc}]{1,255}$`)
)

// HostInput é o que a interface envia. profileId não vem daqui: o override de perfil é
// gerenciado pelo ProfileService.
type HostInput struct {
	Label            string           `json:"label"`
	Address          string           `json:"address"`
	Port             int              `json:"port"`
	Username         string           `json:"username"`
	AuthMethod       model.AuthMethod `json:"authMethod"`
	CredentialID     *string          `json:"credentialId"`
	GroupID          *string          `json:"groupId"`
	LegacyAlgorithms bool             `json:"legacyAlgorithms"`
	Protocol         model.Protocol   `json:"protocol"`
	DeviceIcon       string           `json:"deviceIcon"`
	legacySpecified  bool
}

func (in *HostInput) UnmarshalJSON(data []byte) error {
	type wireInput struct {
		Label            string           `json:"label"`
		Address          string           `json:"address"`
		Port             int              `json:"port"`
		Username         string           `json:"username"`
		AuthMethod       model.AuthMethod `json:"authMethod"`
		CredentialID     *string          `json:"credentialId"`
		GroupID          *string          `json:"groupId"`
		LegacyAlgorithms *bool            `json:"legacyAlgorithms"`
		Protocol         model.Protocol   `json:"protocol"`
		DeviceIcon       string           `json:"deviceIcon"`
	}
	var wire wireInput
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*in = HostInput{
		Label: wire.Label, Address: wire.Address, Port: wire.Port, Username: wire.Username,
		AuthMethod: wire.AuthMethod, CredentialID: wire.CredentialID, GroupID: wire.GroupID,
		Protocol: wire.Protocol, DeviceIcon: wire.DeviceIcon,
	}
	if wire.LegacyAlgorithms != nil {
		in.LegacyAlgorithms = *wire.LegacyAlgorithms
		in.legacySpecified = true
	}
	return nil
}

// WithLegacyDefault aplica o padrão somente quando o JSON de criação omitiu o campo. Valores
// true construídos diretamente em Go continuam sendo tratados como explícitos.
func (in HostInput) WithLegacyDefault(value bool) HostInput {
	if !in.legacySpecified && !in.LegacyAlgorithms {
		in.LegacyAlgorithms = value
	}
	return in
}

type HostView struct {
	model.Host
	CredentialName *string `json:"credentialName"`
}

func NormalizeAddress(address string) string {
	a := strings.TrimSpace(address)
	if strings.HasPrefix(a, "[") && strings.HasSuffix(a, "]") {
		a = a[1 : len(a)-1]
	}
	if ip := net.ParseIP(a); ip != nil && strings.Contains(a, ":") {
		return strings.ToLower(a)
	}
	return a
}

func IsValidAddress(address string) bool {
	if net.ParseIP(address) != nil {
		return true
	}
	return len(address) <= 253 && hostnameRe.MatchString(address)
}

func validateHost(c *model.Content, in HostInput) (HostInput, error) {
	label, err := validate.Text(in.Label, 1, 100)
	if err != nil {
		return in, err
	}
	in.Label = label
	if utf8.RuneCountInString(in.Address) > 255 {
		return in, invalid()
	}
	in.Address = NormalizeAddress(in.Address)
	if !IsValidAddress(in.Address) {
		return in, invalid()
	}
	switch in.Protocol {
	case "", model.ProtocolSSH:
		in.Protocol = model.ProtocolSSH
	case model.ProtocolTelnet:
		// Telnet não tem chave nem agente; com senha, o app responde ao login sozinho. O usuário
		// pode ficar vazio: há equipamento que só pede a senha, e sem senha salva quem
		// pergunta tudo é o equipamento.
		in.LegacyAlgorithms = false
		if in.AuthMethod != model.AuthAsk && in.AuthMethod != model.AuthPassword {
			return in, invalid()
		}
	default:
		return in, invalid()
	}
	if !DeviceIcons[in.DeviceIcon] {
		return in, invalid()
	}
	emptyTelnetUser := in.Protocol == model.ProtocolTelnet && in.Username == ""
	if validate.Int(in.Port, 1, 65535) != nil || (!emptyTelnetUser && !usernameRe.MatchString(in.Username)) {
		return in, invalid()
	}
	if err := validate.OptionalUUID(in.CredentialID); err != nil {
		return in, err
	}
	if err := validate.OptionalUUID(in.GroupID); err != nil {
		return in, err
	}
	switch in.AuthMethod {
	case model.AuthAsk, model.AuthAgent:
		in.CredentialID = nil
	case model.AuthPassword, model.AuthKey:
		want := model.CredentialPassword
		if in.AuthMethod == model.AuthKey {
			want = model.CredentialKey
		}
		if in.CredentialID == nil {
			return in, invalid()
		}
		cr := findCredential(c, *in.CredentialID)
		if cr == nil || cr.Kind != want {
			return in, invalid()
		}
	default:
		return in, invalid()
	}
	if in.GroupID != nil && findGroup(c, *in.GroupID) == nil {
		return in, invalid()
	}
	return in, nil
}

func hostIndex(c *model.Content, id string) int {
	for i := range c.Hosts {
		if c.Hosts[i].ID == id {
			return i
		}
	}
	return -1
}

// DeviceIcons são os ícones de equipamento aceitos; "" é o padrão do protocolo. A lista espelha
// DEVICE_ICONS em frontend/src/shared/model/types.ts.
var DeviceIcons = map[string]bool{"": true, "ssh": true, "telnet": true, "router": true, "bng": true, "switch": true, "olt": true,
	"onu": true, "server": true, "firewall": true, "radio": true, "wifi": true, "edge": true}

func FindHost(c *model.Content, id string) (model.Host, error) {
	i := hostIndex(c, id)
	if i < 0 {
		return model.Host{}, notFound()
	}
	return c.Hosts[i], nil
}

func fromInput(in HostInput) model.Host {
	return model.Host{Label: in.Label, Address: in.Address, Port: in.Port, Username: in.Username,
		AuthMethod: in.AuthMethod, CredentialID: in.CredentialID, GroupID: in.GroupID,
		LegacyAlgorithms: in.LegacyAlgorithms, Protocol: in.Protocol, DeviceIcon: in.DeviceIcon}
}

func CreateHost(c *model.Content, in HostInput, now string) (model.Host, error) {
	v, err := validateHost(c, in)
	if err != nil {
		return model.Host{}, err
	}
	h := fromInput(v)
	h.ID, h.CreatedAt, h.UpdatedAt = NewID(), now, now
	c.Hosts = append(c.Hosts, h)
	return h, nil
}

func UpdateHost(c *model.Content, id string, in HostInput, now string) (model.Host, error) {
	i := hostIndex(c, id)
	if i < 0 {
		return model.Host{}, notFound()
	}
	v, err := validateHost(c, in)
	if err != nil {
		return model.Host{}, err
	}
	cur := c.Hosts[i]
	h := fromInput(v)
	h.ID, h.CreatedAt, h.UpdatedAt, h.ProfileID = id, cur.CreatedAt, now, cur.ProfileID
	c.Hosts[i] = h
	return h, nil
}

// RemoveHost apaga o host e suas regras de encaminhamento; credenciais ficam (podem ser
// compartilhadas). O perfil próprio do host é removido pelo ProfileService.
func RemoveHost(c *model.Content, id string) error {
	if hostIndex(c, id) < 0 {
		return notFound()
	}
	hosts := c.Hosts[:0]
	for _, h := range c.Hosts {
		if h.ID != id {
			hosts = append(hosts, h)
		}
	}
	c.Hosts = hosts
	rules := c.ForwardRules[:0]
	for _, r := range c.ForwardRules {
		if r.HostID != id {
			rules = append(rules, r)
		}
	}
	c.ForwardRules = rules
	return nil
}

func MoveHost(c *model.Content, id string, groupID *string, now string) error {
	i := hostIndex(c, id)
	if i < 0 {
		return notFound()
	}
	if groupID != nil && findGroup(c, *groupID) == nil {
		return invalid()
	}
	c.Hosts[i].GroupID = groupID
	c.Hosts[i].UpdatedAt = now
	return nil
}

const copySuffix = " (cópia)"

// freeBindPort: primeira porta livre depois de from no mesmo endereço de escuta (circular).
func freeBindPort(c *model.Content, bindAddress string, from int) (int, error) {
	used := map[int]bool{}
	for _, r := range c.ForwardRules {
		if r.BindAddress == bindAddress {
			used[r.BindPort] = true
		}
	}
	for i := 1; i < 65535; i++ {
		p := (from-1+i)%65535 + 1
		if !used[p] {
			return p, nil
		}
	}
	return 0, invalid()
}

// DuplicateHost cria cópia editável com a mesma credencial, legacyAlgorithms e profileId,
// e copia as regras de encaminhamento com portas livres (US3-5 da 001).
func DuplicateHost(c *model.Content, id, now string) (model.Host, error) {
	src, err := FindHost(c, id)
	if err != nil {
		return model.Host{}, err
	}
	cp := src
	runes := []rune(src.Label)
	if max := 100 - utf8.RuneCountInString(copySuffix); len(runes) > max {
		runes = runes[:max]
	}
	cp.ID, cp.Label, cp.CreatedAt, cp.UpdatedAt = NewID(), string(runes)+copySuffix, now, now
	c.Hosts = append(c.Hosts, cp)
	var rules []model.ForwardRule
	for _, r := range c.ForwardRules {
		if r.HostID == id {
			rules = append(rules, r)
		}
	}
	for _, r := range rules {
		port, err := freeBindPort(c, r.BindAddress, r.BindPort)
		if err != nil {
			return model.Host{}, err
		}
		r.ID, r.HostID, r.BindPort = NewID(), cp.ID, port
		c.ForwardRules = append(c.ForwardRules, r)
	}
	return cp, nil
}

func ToHostView(c *model.Content, h model.Host) HostView {
	v := HostView{Host: h}
	if h.CredentialID != nil {
		if cr := findCredential(c, *h.CredentialID); cr != nil {
			v.CredentialName = &cr.Name
		}
	}
	return v
}

func ListHostViews(c *model.Content) []HostView {
	out := make([]HostView, 0, len(c.Hosts))
	for _, h := range c.Hosts {
		out = append(out, ToHostView(c, h))
	}
	return out
}
