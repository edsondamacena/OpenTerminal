package vault

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"

	"openterminal/internal/apperr"
	"openterminal/internal/model"
	"openterminal/internal/schemes"
)

const (
	BrandSchemeID          = schemes.BrandSchemeID
	DefaultGlobalProfileID = "00000000-0000-4000-8000-000000000002"
)

type terminalProfileData struct {
	FontFamily        string            `json:"fontFamily"`
	FontSize          float64           `json:"fontSize"`
	FontWeight        int               `json:"fontWeight"`
	FontWeightBold    int               `json:"fontWeightBold"`
	LineHeight        float64           `json:"lineHeight"`
	LetterSpacing     float64           `json:"letterSpacing"`
	Ligatures         bool              `json:"ligatures"`
	CursorStyle       model.CursorStyle `json:"cursorStyle"`
	CursorBlink       bool              `json:"cursorBlink"`
	CursorColor       *string           `json:"cursorColor"`
	Scrollback        int               `json:"scrollback"`
	BackgroundOpacity float64           `json:"backgroundOpacity"`
	Blur              int               `json:"blur"`
	Padding           int               `json:"padding"`
	ColorSchemeID     string            `json:"colorSchemeId"`
}

type colorSchemeData struct {
	Foreground string   `json:"foreground"`
	Background string   `json:"background"`
	Cursor     string   `json:"cursor"`
	Selection  string   `json:"selection"`
	ANSI       []string `json:"ansi"`
}

func brandScheme() model.ColorScheme {
	return schemes.Presets()[0]
}

func defaultGlobalProfile() model.TerminalProfile {
	return model.TerminalProfile{
		ID: DefaultGlobalProfileID, Name: "Padrão", IsGlobal: true,
		FontFamily: "JetBrains Mono", FontSize: 14, FontWeight: 400, FontWeightBold: 700,
		LineHeight: 1.2, LetterSpacing: 0, Ligatures: false,
		CursorStyle: model.CursorBlock, CursorBlink: true, Scrollback: 10_000,
		BackgroundOpacity: 1, Blur: 0, Padding: 12, ColorSchemeID: BrandSchemeID,
	}
}

func seedProfiles(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	fail := func(err error) error { _ = tx.Rollback(); return err }
	if _, err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS terminal_profiles_one_global
		ON terminal_profiles(is_global) WHERE is_global = 1`); err != nil {
		return fail(err)
	}
	// Presets acompanham o app: reabrir atualiza nomes e cores pelos ids estáveis.
	for _, scheme := range schemes.Presets() {
		if err := putColorScheme(tx, scheme); err != nil {
			return fail(err)
		}
	}
	var globals int
	if err := tx.QueryRow(`SELECT count(*) FROM terminal_profiles WHERE is_global = 1`).Scan(&globals); err != nil {
		return fail(err)
	}
	if globals == 0 {
		if err := putTerminalProfile(tx, defaultGlobalProfile()); err != nil {
			return fail(err)
		}
	}
	return tx.Commit()
}

func profileData(p model.TerminalProfile) terminalProfileData {
	return terminalProfileData{p.FontFamily, p.FontSize, p.FontWeight, p.FontWeightBold,
		p.LineHeight, p.LetterSpacing, p.Ligatures, p.CursorStyle, p.CursorBlink,
		p.CursorColor, p.Scrollback, p.BackgroundOpacity, p.Blur, p.Padding, p.ColorSchemeID}
}

type sqlExecer interface {
	Exec(string, ...any) (sql.Result, error)
}

func putTerminalProfile(exec sqlExecer, p model.TerminalProfile) error {
	raw, err := json.Marshal(profileData(p))
	if err != nil {
		return err
	}
	global := 0
	if p.IsGlobal {
		global = 1
	}
	_, err = exec.Exec(`INSERT INTO terminal_profiles (id, name, is_global, data) VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, is_global=excluded.is_global, data=excluded.data`,
		p.ID, p.Name, global, string(raw))
	return err
}

func putColorScheme(exec sqlExecer, s model.ColorScheme) error {
	raw, err := json.Marshal(colorSchemeData{s.Foreground, s.Background, s.Cursor, s.Selection, s.ANSI})
	if err != nil {
		return err
	}
	_, err = exec.Exec(`INSERT INTO color_schemes (id, name, origin, data) VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, origin=excluded.origin, data=excluded.data`,
		s.ID, s.Name, string(s.Origin), string(raw))
	return err
}

func scanProfile(scanner interface{ Scan(...any) error }) (model.TerminalProfile, error) {
	var p model.TerminalProfile
	var global int
	var raw string
	if err := scanner.Scan(&p.ID, &p.Name, &global, &raw); err != nil {
		return p, err
	}
	var d terminalProfileData
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return p, err
	}
	p.IsGlobal = global != 0
	p.FontFamily, p.FontSize, p.FontWeight, p.FontWeightBold = d.FontFamily, d.FontSize, d.FontWeight, d.FontWeightBold
	p.LineHeight, p.LetterSpacing, p.Ligatures = d.LineHeight, d.LetterSpacing, d.Ligatures
	p.CursorStyle, p.CursorBlink, p.CursorColor = d.CursorStyle, d.CursorBlink, d.CursorColor
	p.Scrollback, p.BackgroundOpacity, p.Blur, p.Padding = d.Scrollback, d.BackgroundOpacity, d.Blur, d.Padding
	p.ColorSchemeID = d.ColorSchemeID
	return p, nil
}

func scanScheme(scanner interface{ Scan(...any) error }) (model.ColorScheme, error) {
	var s model.ColorScheme
	var origin, raw string
	if err := scanner.Scan(&s.ID, &s.Name, &origin, &raw); err != nil {
		return s, err
	}
	var d colorSchemeData
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return s, err
	}
	s.Origin = model.SchemeOrigin(origin)
	s.Foreground, s.Background, s.Cursor, s.Selection, s.ANSI = d.Foreground, d.Background, d.Cursor, d.Selection, d.ANSI
	return s, nil
}

func (s *Store) TerminalProfiles() ([]model.TerminalProfile, error) {
	s.mu.Lock()
	defer s.unlockAndNotify()
	rows, err := s.db.Query(`SELECT id, name, is_global, data FROM terminal_profiles ORDER BY is_global DESC, name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.TerminalProfile
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GlobalProfile() (model.TerminalProfile, error) {
	s.mu.Lock()
	defer s.unlockAndNotify()
	return scanProfile(s.db.QueryRow(`SELECT id, name, is_global, data FROM terminal_profiles WHERE is_global = 1`))
}

func (s *Store) SaveGlobalProfile(p model.TerminalProfile) error {
	s.mu.Lock()
	defer s.unlockAndNotify()
	var id string
	if err := s.db.QueryRow(`SELECT id FROM terminal_profiles WHERE is_global = 1`).Scan(&id); err != nil {
		return err
	}
	p.ID, p.IsGlobal = id, true
	return putTerminalProfile(s.db, p)
}

func (s *Store) HostProfile(hostID string) (*model.TerminalProfile, error) {
	s.mu.Lock()
	defer s.unlockAndNotify()
	if s.status.State != StateUnlocked {
		return nil, apperr.New(apperr.VaultLocked)
	}
	var profileID *string
	found := false
	for i := range s.content.Hosts {
		if s.content.Hosts[i].ID == hostID {
			found = true
			profileID = s.content.Hosts[i].ProfileID
			break
		}
	}
	if !found {
		return nil, apperr.New(apperr.NotFound)
	}
	if profileID == nil {
		return nil, nil
	}
	p, err := scanProfile(s.db.QueryRow(`SELECT id, name, is_global, data FROM terminal_profiles WHERE id = ? AND is_global = 0`, *profileID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.New(apperr.NotFound)
	}
	return &p, err
}

func (s *Store) SetHostProfile(hostID string, p *model.TerminalProfile) error {
	s.mu.Lock()
	defer s.unlockAndNotify()
	if s.status.State != StateUnlocked {
		return apperr.New(apperr.VaultLocked)
	}
	draft := s.content.Clone()
	i := -1
	for n := range draft.Hosts {
		if draft.Hosts[n].ID == hostID {
			i = n
			break
		}
	}
	if i < 0 {
		return apperr.New(apperr.NotFound)
	}
	old := draft.Hosts[i].ProfileID
	if p == nil {
		draft.Hosts[i].ProfileID = nil
	} else {
		draft.Hosts[i].ProfileID = model.Ptr(p.ID)
	}
	pre := func(tx *sql.Tx) error {
		if p == nil {
			if old != nil {
				_, err := tx.Exec(`DELETE FROM terminal_profiles WHERE id = ? AND is_global = 0`, *old)
				return err
			}
			return nil
		}
		if old != nil && *old != p.ID {
			return apperr.New(apperr.InvalidInput)
		}
		return putTerminalProfile(tx, *p)
	}
	sigs, err := s.persist(pre, s.dek, s.sigs, draft)
	if err != nil {
		return err
	}
	s.content, s.sigs = draft, sigs
	return nil
}

func (s *Store) ColorSchemes() ([]model.ColorScheme, error) {
	s.mu.Lock()
	defer s.unlockAndNotify()
	rows, err := s.db.Query(`SELECT id, name, origin, data FROM color_schemes ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ColorScheme
	for rows.Next() {
		item, err := scanScheme(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) SaveColorScheme(item model.ColorScheme) error {
	s.mu.Lock()
	defer s.unlockAndNotify()
	var existingID string
	err := s.db.QueryRow(`SELECT id FROM color_schemes WHERE name = ?`, item.Name).Scan(&existingID)
	if err == nil && existingID != item.ID {
		return &apperr.Error{Code: apperr.AlreadyExists, Message: apperr.Messages[apperr.AlreadyExists],
			Detail: "já existe um esquema com o nome " + strconv.Quote(item.Name)}
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err := putColorScheme(s.db, item); err != nil {
		return apperr.New(apperr.InvalidInput)
	}
	return nil
}

func (s *Store) DeleteColorScheme(id string) error {
	s.mu.Lock()
	defer s.unlockAndNotify()
	rows, err := s.db.Query(`SELECT id, name, is_global, data FROM terminal_profiles`)
	if err != nil {
		return err
	}
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			_ = rows.Close()
			return err
		}
		if p.ColorSchemeID == id {
			_ = rows.Close()
			return apperr.New(apperr.InUse)
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	res, err := s.db.Exec(`DELETE FROM color_schemes WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return apperr.New(apperr.NotFound)
	}
	return nil
}

// ApplyLegacyTerminalFont conclui a migração adiada do settings.json da versão 001.
// A preferência temporária só é removida na mesma transação que atualiza o perfil global.
func (s *Store) ApplyLegacyTerminalFont(family string, size int) error {
	s.mu.Lock()
	defer s.unlockAndNotify()
	p, err := scanProfile(s.db.QueryRow(`SELECT id, name, is_global, data FROM terminal_profiles WHERE is_global = 1`))
	if err != nil {
		return err
	}
	p.FontFamily, p.FontSize = family, float64(size)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := putTerminalProfile(tx, p); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`DELETE FROM preferences WHERE key = 'legacyTerminalFont'`); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
