package vault

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"openterminal/internal/model"
)

// Cada tabela cifrada tem colunas estruturais em claro e o JSON da entidade em `sealed`
// (nonce ‖ XChaCha20-Poly1305(DEK, JSON), AD = "<tabela>:<id>"), conforme data-model.md.

type row struct {
	cols  []any
	plain []byte
}

type groupPlain struct {
	Name string `json:"name"`
}

type hostPlain struct {
	Label            string           `json:"label"`
	Address          string           `json:"address"`
	Port             int              `json:"port"`
	Username         string           `json:"username"`
	AuthMethod       model.AuthMethod `json:"authMethod"`
	CredentialID     *string          `json:"credentialId"`
	LegacyAlgorithms bool             `json:"legacyAlgorithms"`
	ProfileID        *string          `json:"profileId"`
	// Vazios ficam fora do JSON: linhas antigas continuam com a mesma assinatura.
	Protocol   model.Protocol `json:"protocol,omitempty"`
	DeviceIcon string         `json:"deviceIcon,omitempty"`
}

type credentialPlain struct {
	Name        string  `json:"name"`
	Password    string  `json:"password,omitempty"`
	PrivateKey  string  `json:"privateKey,omitempty"`
	Passphrase  *string `json:"passphrase,omitempty"`
	KeyType     string  `json:"keyType,omitempty"`
	PublicKey   string  `json:"publicKey,omitempty"`
	Fingerprint string  `json:"fingerprint,omitempty"`
	Origin      string  `json:"origin,omitempty"`
}

type knownPlain struct {
	Endpoint    string `json:"endpoint"`
	KeyType     string `json:"keyType"`
	Fingerprint string `json:"fingerprint"`
	AcceptedAt  string `json:"acceptedAt"`
}

type forwardPlain struct {
	Label            *string `json:"label"`
	BindAddress      string  `json:"bindAddress"`
	AllowNonLoopback bool    `json:"allowNonLoopback"`
	BindPort         int     `json:"bindPort"`
	DestHost         string  `json:"destHost"`
	DestPort         int     `json:"destPort"`
}

type table struct {
	name string
	cols []string
}

// Ordem de gravação; as FKs são verificadas só no commit (defer_foreign_keys).
var tables = []table{
	{"groups", []string{"parent_id", "sort_order"}},
	{"credentials", []string{"kind", "created_at"}},
	{"hosts", []string{"group_id", "sort_order", "created_at", "updated_at"}},
	{"known_hosts", nil},
	{"forward_rules", []string{"host_id"}},
	{"vault_settings", nil},
}

const settingsID = "1"

func nullable(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func encodeContent(c *model.Content) (map[string]map[string]row, error) {
	out := map[string]map[string]row{}
	put := func(t, id string, cols []any, v any) error {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		if out[t] == nil {
			out[t] = map[string]row{}
		}
		if _, dup := out[t][id]; dup {
			return fmt.Errorf("id repetido em %s", t)
		}
		out[t][id] = row{cols: cols, plain: b}
		return nil
	}
	for _, g := range c.Groups {
		if err := put("groups", g.ID, []any{nullable(g.ParentID), g.SortOrder}, groupPlain{g.Name}); err != nil {
			return nil, err
		}
	}
	for _, cr := range c.Credentials {
		p := credentialPlain{Name: cr.Name, Password: cr.Password, PrivateKey: cr.PrivateKey,
			Passphrase: cr.Passphrase, KeyType: cr.KeyType, PublicKey: cr.PublicKey,
			Fingerprint: cr.Fingerprint, Origin: cr.Origin}
		if err := put("credentials", cr.ID, []any{string(cr.Kind), cr.CreatedAt}, p); err != nil {
			return nil, err
		}
	}
	for _, h := range c.Hosts {
		p := hostPlain{Label: h.Label, Address: h.Address, Port: h.Port, Username: h.Username,
			AuthMethod: h.AuthMethod, CredentialID: h.CredentialID,
			LegacyAlgorithms: h.LegacyAlgorithms, ProfileID: h.ProfileID,
			Protocol: h.Protocol, DeviceIcon: h.DeviceIcon}
		if err := put("hosts", h.ID, []any{nullable(h.GroupID), 0, h.CreatedAt, h.UpdatedAt}, p); err != nil {
			return nil, err
		}
	}
	for _, k := range c.KnownHosts {
		if err := put("known_hosts", k.ID, nil, knownPlain{k.Endpoint, k.KeyType, k.Fingerprint, k.AcceptedAt}); err != nil {
			return nil, err
		}
	}
	for _, r := range c.ForwardRules {
		p := forwardPlain{r.Label, r.BindAddress, r.AllowNonLoopback, r.BindPort, r.DestHost, r.DestPort}
		if err := put("forward_rules", r.ID, []any{r.HostID}, p); err != nil {
			return nil, err
		}
	}
	if err := put("vault_settings", settingsID, nil, c.Settings); err != nil {
		return nil, err
	}
	return out, nil
}

func ad(t, id string) []byte { return []byte(t + ":" + id) }

func signature(r row) string {
	h := sha256.New()
	cols, _ := json.Marshal(r.cols)
	h.Write(cols)
	h.Write([]byte{0})
	h.Write(r.plain)
	return string(h.Sum(nil))
}

func upsertSQL(t table) string {
	names := append([]string{"id"}, t.cols...)
	names = append(names, "sealed")
	marks := strings.TrimSuffix(strings.Repeat("?, ", len(names)), ", ")
	var set []string
	for _, c := range append(append([]string{}, t.cols...), "sealed") {
		set = append(set, c+" = excluded."+c)
	}
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT(id) DO UPDATE SET %s",
		t.name, strings.Join(names, ", "), marks, strings.Join(set, ", "))
}

// persist grava c comparando com as assinaturas anteriores (old pode ser nil: grava tudo).
// pre roda dentro da mesma transação, antes das linhas.
func (s *Store) persist(pre func(*sql.Tx) error, dek []byte, old map[string]map[string]string, c *model.Content) (map[string]map[string]string, error) {
	rows, err := encodeContent(c)
	if err != nil {
		return nil, err
	}
	defer func() {
		for _, rs := range rows {
			for _, r := range rs {
				Zero(r.plain)
			}
		}
	}()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	fail := func(err error) (map[string]map[string]string, error) {
		_ = tx.Rollback()
		return nil, err
	}
	if _, err := tx.Exec(`PRAGMA defer_foreign_keys = ON`); err != nil {
		return fail(err)
	}
	if pre != nil {
		if err := pre(tx); err != nil {
			return fail(err)
		}
	}
	sigs := map[string]map[string]string{}
	for _, t := range tables {
		sigs[t.name] = map[string]string{}
		upsert := upsertSQL(t)
		for id, r := range rows[t.name] {
			sig := signature(r)
			sigs[t.name][id] = sig
			if old != nil && old[t.name][id] == sig {
				continue
			}
			sealed, err := Seal(dek, r.plain, ad(t.name, id))
			if err != nil {
				return fail(err)
			}
			args := append(append([]any{id}, r.cols...), sealed)
			if _, err := tx.Exec(upsert, args...); err != nil {
				return fail(err)
			}
		}
		for id := range old[t.name] {
			if _, keep := rows[t.name][id]; keep {
				continue
			}
			if _, err := tx.Exec(`DELETE FROM `+t.name+` WHERE id = ?`, id); err != nil {
				return fail(err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return sigs, nil
}

func str(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	case int64:
		return fmt.Sprint(x)
	}
	return ""
}

func strPtr(v any) *string {
	if v == nil {
		return nil
	}
	s := str(v)
	return &s
}

func intOf(v any) int {
	if x, ok := v.(int64); ok {
		return int(x)
	}
	return 0
}

// loadContent decifra todas as tabelas. Qualquer linha que não abra torna o cofre ilegível.
func loadContent(db *sql.DB, dek []byte) (*model.Content, map[string]map[string]string, error) {
	c := model.EmptyContent()
	sigs := map[string]map[string]string{}
	settingsFound := false
	for _, t := range tables {
		sigs[t.name] = map[string]string{}
		sel := append([]string{"CAST(id AS TEXT)"}, t.cols...)
		sel = append(sel, "sealed")
		rs, err := db.Query(fmt.Sprintf("SELECT %s FROM %s ORDER BY rowid", strings.Join(sel, ", "), t.name))
		if err != nil {
			return nil, nil, err
		}
		for rs.Next() {
			var id string
			var sealed []byte
			cols := make([]any, len(t.cols))
			dest := []any{&id}
			for i := range cols {
				dest = append(dest, &cols[i])
			}
			dest = append(dest, &sealed)
			if err := rs.Scan(dest...); err != nil {
				_ = rs.Close()
				return nil, nil, err
			}
			plain, err := Unseal(dek, sealed, ad(t.name, id))
			if err != nil {
				_ = rs.Close()
				return nil, nil, err
			}
			err = decodeRow(c, t.name, id, cols, plain)
			sigs[t.name][id] = signature(row{cols: normalizeCols(t.name, cols), plain: plain})
			Zero(plain)
			if err != nil {
				_ = rs.Close()
				return nil, nil, err
			}
			if t.name == "vault_settings" {
				settingsFound = true
			}
		}
		if err := rs.Close(); err != nil {
			return nil, nil, err
		}
	}
	if !settingsFound {
		return nil, nil, fmt.Errorf("vault_settings ausente")
	}
	return c, sigs, nil
}

// normalizeCols deixa os valores lidos do banco com os mesmos tipos usados na gravação, para
// que a assinatura de uma linha não mude só por ter sido lida.
func normalizeCols(t string, cols []any) []any {
	out := make([]any, len(cols))
	for i, v := range cols {
		switch x := v.(type) {
		case int64:
			out[i] = int(x)
		case []byte:
			out[i] = string(x)
		default:
			out[i] = v
		}
	}
	return out
}

func decodeRow(c *model.Content, t, id string, cols []any, plain []byte) error {
	switch t {
	case "groups":
		var p groupPlain
		if err := json.Unmarshal(plain, &p); err != nil {
			return err
		}
		c.Groups = append(c.Groups, model.Group{ID: id, Name: p.Name, ParentID: strPtr(cols[0]), SortOrder: intOf(cols[1])})
	case "credentials":
		var p credentialPlain
		if err := json.Unmarshal(plain, &p); err != nil {
			return err
		}
		c.Credentials = append(c.Credentials, model.Credential{ID: id, Kind: model.CredentialKind(str(cols[0])),
			CreatedAt: str(cols[1]), Name: p.Name, Password: p.Password, PrivateKey: p.PrivateKey,
			Passphrase: p.Passphrase, KeyType: p.KeyType, PublicKey: p.PublicKey,
			Fingerprint: p.Fingerprint, Origin: p.Origin})
	case "hosts":
		var p hostPlain
		if err := json.Unmarshal(plain, &p); err != nil {
			return err
		}
		c.Hosts = append(c.Hosts, model.Host{ID: id, GroupID: strPtr(cols[0]), CreatedAt: str(cols[2]),
			UpdatedAt: str(cols[3]), Label: p.Label, Address: p.Address, Port: p.Port,
			Username: p.Username, AuthMethod: p.AuthMethod, CredentialID: p.CredentialID,
			LegacyAlgorithms: p.LegacyAlgorithms, ProfileID: p.ProfileID,
			Protocol: p.Protocol, DeviceIcon: p.DeviceIcon})
	case "known_hosts":
		var p knownPlain
		if err := json.Unmarshal(plain, &p); err != nil {
			return err
		}
		c.KnownHosts = append(c.KnownHosts, model.KnownHost{ID: id, Endpoint: p.Endpoint,
			KeyType: p.KeyType, Fingerprint: p.Fingerprint, AcceptedAt: p.AcceptedAt})
	case "forward_rules":
		var p forwardPlain
		if err := json.Unmarshal(plain, &p); err != nil {
			return err
		}
		c.ForwardRules = append(c.ForwardRules, model.ForwardRule{ID: id, HostID: str(cols[0]),
			Label: p.Label, BindAddress: p.BindAddress, AllowNonLoopback: p.AllowNonLoopback,
			BindPort: p.BindPort, DestHost: p.DestHost, DestPort: p.DestPort})
	case "vault_settings":
		var p model.VaultSettings
		if err := json.Unmarshal(plain, &p); err != nil {
			return err
		}
		c.Settings = model.NormalizeVaultSettings(p)
	}
	return nil
}
