package vault

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite"

	"openterminal/internal/apperr"
	"openterminal/internal/model"
)

const (
	DBFile     = "openterminal.db"
	LegacyFile = "vault.svlt"

	MinPasswordLength = 8
	maxPasswordLength = 1024
	formatVersion     = 1
	kdfAlg            = "argon2id"
)

type State string

const (
	StateNonexistent State = "nonexistent"
	StateLocked      State = "locked"
	StateUnlocked    State = "unlocked"
	StateUnreadable  State = "unreadable"
	// StateMigratable: existe o vault.svlt da 001 e ainda não há cofre no banco (FR-102).
	StateMigratable State = "migratable"
)

const (
	ReasonCorrupted          = "corrupted"
	ReasonUnsupportedVersion = "unsupportedVersion"
)

type Status struct {
	State  State   `json:"state"`
	Reason *string `json:"reason"`
}

type meta struct {
	time       uint32
	memoryKiB  uint32
	salt       []byte
	wrappedDEK []byte
	createdAt  string
}

func (m meta) kdf() KDFParams {
	return KDFParams{Time: m.time, MemoryKiB: m.memoryKiB, Salt: m.salt}
}

// wrapAD autentica os parâmetros do KDF e a data de criação junto com a DEK embrulhada.
// O prefixo "shellvault:dek:v1" é do formato do arquivo, não da marca: trocá-lo deixa todos os
// cofres existentes sem abrir.
func (m meta) wrapAD() []byte {
	return fmt.Appendf(nil, "shellvault:dek:v1|%s|%d|%d|%x|%s", kdfAlg, m.time, m.memoryKiB, m.salt, m.createdAt)
}

// Store é o cofre. O banco fica aberto desde o início (preferências e perfis não dependem do
// cofre); as tabelas cifradas só são lidas e escritas com o cofre desbloqueado.
// Os ouvintes de OnStatus rodam depois que o Store libera o lock e podem chamá-lo de volta.
type Store struct {
	mu        sync.Mutex
	dir       string
	kdf       func(salt []byte) KDFParams
	now       func() string
	db        *sql.DB
	status    Status
	dek       []byte
	meta      *meta
	content   *model.Content
	sigs      map[string]map[string]string
	listeners []func(Status)
	pending   []Status
	// Mudanças locais do conteúdo (o sync na nuvem escuta); Replace não avisa.
	changeListeners []func()
	changed         bool
}

// Open abre (ou cria) o banco em dir. kdf define o custo do Argon2id para cofres novos e trocas
// de senha (DefaultKDF em produção, TestKDF nos testes).
func Open(dir string, kdf func(salt []byte) KDFParams) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, kdf: kdf, now: nowISO}
	db, err := openDB(filepath.Join(dir, DBFile))
	if err != nil {
		var newer errNewerSchema
		reason := ReasonCorrupted
		if errors.As(err, &newer) {
			reason = ReasonUnsupportedVersion
		}
		s.status = Status{State: StateUnreadable, Reason: &reason}
		return s, nil
	}
	if err := seedProfiles(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	s.db = db
	s.status = s.probe()
	return s, nil
}

func openDB(path string) (*sql.DB, error) {
	dsn := filepath.ToSlash(path) + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(FULL)&_pragma=secure_delete(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// Uma conexão: pragmas por conexão valem sempre e as escritas ficam serializadas.
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func (s *Store) probe() Status {
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM vault_meta`).Scan(&n); err != nil {
		r := ReasonCorrupted
		return Status{State: StateUnreadable, Reason: &r}
	}
	if n > 0 {
		return Status{State: StateLocked}
	}
	return probeLegacy(s.dir)
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.unlockAndNotify()
	s.lockLocked()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func (s *Store) Dir() string { return s.dir }

func (s *Store) Status() Status {
	s.mu.Lock()
	defer s.unlockAndNotify()
	return s.status
}

func (s *Store) OnStatus(fn func(Status)) {
	s.mu.Lock()
	defer s.unlockAndNotify()
	s.listeners = append(s.listeners, fn)
}

// OnChange avisa depois de cada Mutate que gravou algo. Roda fora do lock.
func (s *Store) OnChange(fn func()) {
	s.mu.Lock()
	defer s.unlockAndNotify()
	s.changeListeners = append(s.changeListeners, fn)
}

func (s *Store) setStatus(st Status) {
	s.status = st
	s.pending = append(s.pending, st)
}

// unlockAndNotify libera o lock e só então avisa os ouvintes das mudanças de estado.
func (s *Store) unlockAndNotify() {
	pending, listeners := s.pending, append([]func(Status){}, s.listeners...)
	changed, changeListeners := s.changed, append([]func(){}, s.changeListeners...)
	s.pending, s.changed = nil, false
	s.mu.Unlock()
	for _, st := range pending {
		for _, l := range listeners {
			l(st)
		}
	}
	if changed {
		for _, l := range changeListeners {
			l()
		}
	}
}

func validateNewPassword(password, confirmation string) error {
	n := utf8.RuneCountInString(password)
	if n < MinPasswordLength {
		return apperr.New(apperr.WeakPassword)
	}
	if n > maxPasswordLength || password != confirmation {
		return apperr.New(apperr.InvalidInput)
	}
	return nil
}

func (s *Store) newMeta(password string, dek []byte, createdAt string) (*meta, error) {
	p := s.kdf(RandomSalt())
	m := &meta{time: p.Time, memoryKiB: p.MemoryKiB, salt: p.Salt, createdAt: createdAt}
	kek := DeriveKEK([]byte(password), p)
	defer Zero(kek)
	wrapped, err := Seal(kek, dek, m.wrapAD())
	if err != nil {
		return nil, err
	}
	m.wrappedDEK = wrapped
	return m, nil
}

func (s *Store) loadMeta() (*meta, error) {
	m := &meta{}
	var alg string
	var version int
	err := s.db.QueryRow(`SELECT format_version, kdf_alg, kdf_time, kdf_memory_kib, kdf_salt,
		wrapped_dek, created_at FROM vault_meta WHERE id = 1`).
		Scan(&version, &alg, &m.time, &m.memoryKiB, &m.salt, &m.wrappedDEK, &m.createdAt)
	if err != nil {
		return nil, err
	}
	if version != formatVersion || alg != kdfAlg {
		return nil, errors.New("vault_meta inválido")
	}
	return m, nil
}

// unwrap: senha errada e cabeçalho adulterado dão o mesmo erro (contrato).
func unwrap(password string, m *meta) ([]byte, error) {
	kek := DeriveKEK([]byte(password), m.kdf())
	defer Zero(kek)
	dek, err := Unseal(kek, m.wrappedDEK, m.wrapAD())
	if err != nil {
		return nil, apperr.New(apperr.VaultBadPassword)
	}
	return dek, nil
}

func writeMeta(tx *sql.Tx, m *meta, migratedFrom *string) error {
	_, err := tx.Exec(`INSERT INTO vault_meta (id, format_version, kdf_alg, kdf_time, kdf_memory_kib,
		kdf_salt, wrapped_dek, created_at, migrated_from) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET kdf_time = excluded.kdf_time,
		kdf_memory_kib = excluded.kdf_memory_kib, kdf_salt = excluded.kdf_salt,
		wrapped_dek = excluded.wrapped_dek`,
		formatVersion, kdfAlg, m.time, m.memoryKiB, m.salt, m.wrappedDEK, m.createdAt, migratedFrom)
	return err
}

func (s *Store) Create(password, confirmation string) error {
	return s.create(password, confirmation, model.EmptyContent(), nil)
}

// ImportMigrated cria o cofre com o conteúdo vindo da 001, numa única transação (FR-102).
// Só vale no estado migratable; o chamador renomeia o vault.svlt depois do sucesso.
func (s *Store) ImportMigrated(password string, content *model.Content, from string) error {
	return s.create(password, password, content, &from)
}

func (s *Store) create(password, confirmation string, content *model.Content, migratedFrom *string) error {
	s.mu.Lock()
	defer s.unlockAndNotify()
	switch s.status.State {
	case StateNonexistent:
		if migratedFrom != nil {
			return apperr.New(apperr.InvalidInput)
		}
	case StateMigratable:
		if migratedFrom == nil {
			return apperr.New(apperr.VaultExists)
		}
	case StateUnreadable:
		return apperr.New(apperr.VaultUnreadable)
	default:
		return apperr.New(apperr.VaultExists)
	}
	if err := validateNewPassword(password, confirmation); err != nil {
		return err
	}
	dek := RandomKey()
	m, err := s.newMeta(password, dek, s.now())
	if err != nil {
		Zero(dek)
		return err
	}
	sigs, err := s.persist(func(tx *sql.Tx) error { return writeMeta(tx, m, migratedFrom) }, dek, nil, content)
	if err != nil {
		Zero(dek)
		return err
	}
	s.dek, s.meta, s.content, s.sigs = dek, m, content.Clone(), sigs
	s.setStatus(Status{State: StateUnlocked})
	return nil
}

func (s *Store) Unlock(password string) error {
	s.mu.Lock()
	defer s.unlockAndNotify()
	switch s.status.State {
	case StateUnlocked:
		return nil
	case StateNonexistent, StateMigratable:
		return apperr.New(apperr.VaultNotFound)
	case StateUnreadable:
		return apperr.New(apperr.VaultUnreadable)
	}
	m, err := s.loadMeta()
	if err != nil {
		return s.corrupted()
	}
	dek, err := unwrap(password, m)
	if err != nil {
		return err
	}
	content, sigs, err := loadContent(s.db, dek)
	if err != nil {
		Zero(dek)
		return s.corrupted()
	}
	s.dek, s.meta, s.content, s.sigs = dek, m, content, sigs
	s.setStatus(Status{State: StateUnlocked})
	return nil
}

// CheckPassword confere a senha mestra sem mudar o estado (o sync pede a senha ao conectar).
func (s *Store) CheckPassword(password string) error {
	s.mu.Lock()
	defer s.unlockAndNotify()
	if s.status.State != StateUnlocked || s.meta == nil {
		return apperr.New(apperr.VaultLocked)
	}
	dek, err := unwrap(password, s.meta)
	if err != nil {
		return err
	}
	Zero(dek)
	return nil
}

const dekCheckPref = "vault.dekCheck"

// RememberDEK devolve uma cópia da chave de dados para o desbloqueio automático e grava um
// verificador dela nas preferências. Só o chaveiro do sistema deve guardar a cópia.
func (s *Store) RememberDEK() ([]byte, error) {
	s.mu.Lock()
	dek := append([]byte(nil), s.dek...)
	unlocked := s.status.State == StateUnlocked && s.dek != nil
	s.mu.Unlock()
	if !unlocked {
		return nil, apperr.New(apperr.VaultLocked)
	}
	check, err := Seal(dek, nil, []byte(dekCheckPref))
	if err != nil {
		return nil, err
	}
	if err := s.SetPreference(dekCheckPref, check); err != nil {
		return nil, err
	}
	return dek, nil
}

// UnlockWithDEK destrava com a chave guardada pelo desbloqueio automático. O verificador
// impede usar uma chave de outro cofre (com o cofre vazio, nada mais acusaria o erro e os
// dados novos seriam gravados com a chave errada).
func (s *Store) UnlockWithDEK(dek []byte) error {
	var check []byte
	if ok, err := s.Preference(dekCheckPref, &check); err != nil || !ok {
		return apperr.New(apperr.VaultBadPassword)
	}
	if _, err := Unseal(dek, check, []byte(dekCheckPref)); err != nil {
		return apperr.New(apperr.VaultBadPassword)
	}
	s.mu.Lock()
	defer s.unlockAndNotify()
	if s.status.State == StateUnlocked {
		return nil
	}
	if s.status.State != StateLocked {
		return apperr.New(apperr.VaultNotFound)
	}
	m, err := s.loadMeta()
	if err != nil {
		return s.corrupted()
	}
	own := append([]byte(nil), dek...)
	content, sigs, err := loadContent(s.db, own)
	if err != nil {
		Zero(own)
		return s.corrupted()
	}
	s.dek, s.meta, s.content, s.sigs = own, m, content, sigs
	s.setStatus(Status{State: StateUnlocked})
	return nil
}

func (s *Store) corrupted() error {
	r := ReasonCorrupted
	s.setStatus(Status{State: StateUnreadable, Reason: &r})
	return apperr.New(apperr.VaultUnreadable)
}

// Lock descarta DEK e conteúdo decifrado e esvazia o WAL.
func (s *Store) Lock() {
	s.mu.Lock()
	defer s.unlockAndNotify()
	s.lockLocked()
}

func (s *Store) lockLocked() {
	wasUnlocked := s.status.State == StateUnlocked
	Zero(s.dek)
	s.dek, s.meta, s.content, s.sigs = nil, nil, nil, nil
	if s.db != nil {
		_, _ = s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	}
	if wasUnlocked {
		s.setStatus(Status{State: StateLocked})
	}
}

// ChangePassword re-embrulha a DEK com KEK nova (sal novo); as linhas não são recifradas.
func (s *Store) ChangePassword(current, next, confirmation string) error {
	s.mu.Lock()
	defer s.unlockAndNotify()
	if s.status.State != StateUnlocked {
		return apperr.New(apperr.VaultLocked)
	}
	check, err := unwrap(current, s.meta)
	if err != nil {
		return err
	}
	Zero(check)
	if err := validateNewPassword(next, confirmation); err != nil {
		return err
	}
	m, err := s.newMeta(next, s.dek, s.meta.createdAt)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := writeMeta(tx, m, nil); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.meta = m
	return nil
}

// Erase apaga o cofre; preferências, perfis e esquemas de cor (sem segredos) ficam.
func (s *Store) Erase() error {
	s.mu.Lock()
	defer s.unlockAndNotify()
	if s.db == nil {
		return apperr.New(apperr.VaultUnreadable)
	}
	Zero(s.dek)
	s.dek, s.meta, s.content, s.sigs = nil, nil, nil, nil
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for _, t := range []string{"forward_rules", "hosts", "known_hosts", "credentials", "groups", "vault_settings", "vault_meta"} {
		if _, err := tx.Exec(`DELETE FROM ` + t); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	_, _ = s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	s.setStatus(s.probe())
	return nil
}

// Read devolve uma cópia do conteúdo decifrado.
func (s *Store) Read() (*model.Content, error) {
	s.mu.Lock()
	defer s.unlockAndNotify()
	if s.status.State != StateUnlocked {
		return nil, apperr.New(apperr.VaultLocked)
	}
	return s.content.Clone(), nil
}

// Mutate aplica fn a um rascunho e grava, numa transação, só as linhas que mudaram.
// Se fn ou a gravação falharem, nada muda.
func (s *Store) Mutate(fn func(c *model.Content) error) error {
	return s.mutate(fn, true)
}

// Replace é o Mutate do sync: grava o conteúdo vindo da nuvem sem avisar OnChange, para não
// devolver à nuvem o que acabou de chegar dela.
func (s *Store) Replace(fn func(c *model.Content) error) error {
	return s.mutate(fn, false)
}

func (s *Store) mutate(fn func(c *model.Content) error, notify bool) error {
	s.mu.Lock()
	defer s.unlockAndNotify()
	if s.status.State != StateUnlocked {
		return apperr.New(apperr.VaultLocked)
	}
	draft := s.content.Clone()
	if err := fn(draft); err != nil {
		return err
	}
	sigs, err := s.persist(nil, s.dek, s.sigs, draft)
	if err != nil {
		return err
	}
	// Mutate sem diferença (ex.: salvar sem mudar nada) não conta como mudança.
	if notify && !sameSigs(s.sigs, sigs) {
		s.changed = true
	}
	s.content, s.sigs = draft, sigs
	return nil
}

func sameSigs(a, b map[string]map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for t, rows := range a {
		other := b[t]
		if len(rows) != len(other) {
			return false
		}
		for id, sig := range rows {
			if other[id] != sig {
				return false
			}
		}
	}
	return true
}

// Preference lê uma preferência (sem segredos; funciona com o cofre bloqueado).
func (s *Store) Preference(key string, out any) (bool, error) {
	s.mu.Lock()
	defer s.unlockAndNotify()
	if s.db == nil {
		return false, apperr.New(apperr.VaultUnreadable)
	}
	var raw string
	err := s.db.QueryRow(`SELECT value FROM preferences WHERE key = ?`, key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal([]byte(raw), out)
}

func (s *Store) SetPreference(key string, value any) error {
	s.mu.Lock()
	defer s.unlockAndNotify()
	if s.db == nil {
		return apperr.New(apperr.VaultUnreadable)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO preferences (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, string(raw))
	return err
}

// SealSecret cifra um segredo deste aparelho (ex.: o token do Google) com a chave de dados do
// cofre. O resultado vai nas preferências: fica fora do conteúdo, do backup e do sync, e
// continua válido depois de trocar a senha mestra (a troca só reembrulha a chave de dados).
func (s *Store) SealSecret(label string, plain []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.unlockAndNotify()
	if s.status.State != StateUnlocked || s.dek == nil {
		return nil, apperr.New(apperr.VaultLocked)
	}
	return Seal(s.dek, plain, secretAD(label))
}

// OpenSecret desfaz SealSecret; o label tem de ser o mesmo.
func (s *Store) OpenSecret(label string, sealed []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.unlockAndNotify()
	if s.status.State != StateUnlocked || s.dek == nil {
		return nil, apperr.New(apperr.VaultLocked)
	}
	return Unseal(s.dek, sealed, secretAD(label))
}

func secretAD(label string) []byte { return []byte("openterminal:secret:v1|" + label) }

func nowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}
