package vault

import (
	"database/sql"
	"fmt"
)

// SchemaVersion é o PRAGMA user_version que esta versão sabe ler. Toda mudança de esquema
// ganha uma migração numerada com teste (Princípio III).
const SchemaVersion = 1

// migrations[i] leva o banco da versão i para i+1.
var migrations = []string{
	// 0 → 1 (data-model.md da 002)
	`
CREATE TABLE vault_meta (
	id             INTEGER PRIMARY KEY CHECK (id = 1),
	format_version INTEGER NOT NULL,
	kdf_alg        TEXT    NOT NULL,
	kdf_time       INTEGER NOT NULL,
	kdf_memory_kib INTEGER NOT NULL,
	kdf_salt       BLOB    NOT NULL,
	wrapped_dek    BLOB    NOT NULL,
	created_at     TEXT    NOT NULL,
	migrated_from  TEXT
);
CREATE TABLE groups (
	id         TEXT PRIMARY KEY,
	parent_id  TEXT REFERENCES groups(id),
	sort_order INTEGER NOT NULL,
	sealed     BLOB NOT NULL
);
CREATE TABLE credentials (
	id         TEXT PRIMARY KEY,
	kind       TEXT NOT NULL,
	created_at TEXT NOT NULL,
	sealed     BLOB NOT NULL
);
CREATE TABLE hosts (
	id         TEXT PRIMARY KEY,
	group_id   TEXT REFERENCES groups(id),
	sort_order INTEGER NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	sealed     BLOB NOT NULL
);
CREATE TABLE known_hosts (
	id     TEXT PRIMARY KEY,
	sealed BLOB NOT NULL
);
CREATE TABLE forward_rules (
	id      TEXT PRIMARY KEY,
	host_id TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
	sealed  BLOB NOT NULL
);
CREATE TABLE vault_settings (
	id     INTEGER PRIMARY KEY CHECK (id = 1),
	sealed BLOB NOT NULL
);
CREATE TABLE preferences (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
CREATE TABLE terminal_profiles (
	id        TEXT PRIMARY KEY,
	name      TEXT NOT NULL,
	is_global INTEGER NOT NULL,
	data      TEXT NOT NULL
);
CREATE TABLE color_schemes (
	id     TEXT PRIMARY KEY,
	name   TEXT NOT NULL UNIQUE,
	origin TEXT NOT NULL,
	data   TEXT NOT NULL
);
`,
}

// errNewerSchema: banco criado por uma versão mais nova do app.
type errNewerSchema struct{ version int }

func (e errNewerSchema) Error() string {
	return fmt.Sprintf("esquema %d é mais novo que o suportado (%d)", e.version, SchemaVersion)
}

func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version > SchemaVersion {
		return errNewerSchema{version}
	}
	for v := version; v < SchemaVersion; v++ {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[v]); err != nil {
			_ = tx.Rollback()
			return err
		}
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, v+1)); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
