package library

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/lizhian/viva-agents-manager/internal/domain"
)

// schemaVersion is how many migrations have been applied. It lives in SQLite's
// own user_version, so the file itself says which shape it is in.
const schemaVersion = 1

// migrations are applied in order, each raising user_version by one. They only
// ever add: an older application opening a newer file refuses to run it rather
// than rewriting it.
var migrations = []func(*sql.DB) error{
	func(db *sql.DB) error {
		_, err := db.Exec(`
CREATE TABLE skill_sources (
	id INTEGER PRIMARY KEY,
	safe TEXT NOT NULL UNIQUE,
	installed_at INTEGER NOT NULL
);
CREATE TABLE skills (
	id INTEGER PRIMARY KEY,
	source_id INTEGER NOT NULL REFERENCES skill_sources(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	installed_at INTEGER NOT NULL,
	UNIQUE(source_id, name)
);`)
		return err
	},
}

// openIndex opens the SQLite file and brings it up to the current schema.
func openIndex(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// One connection: the index is a cache of a few hundred rows, and a single
	// writer keeps the rebuilds from queueing on each other.
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{
		`PRAGMA journal_mode = WAL`,
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA foreign_keys = ON`,
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, err
		}
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// migrate applies whatever the file is missing, one version at a time.
func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version > schemaVersion {
		return fmt.Errorf("library: 索引版本 %d 比程序认识的 %d 更新", version, schemaVersion)
	}
	for ; version < schemaVersion; version++ {
		if err := migrations[version](db); err != nil {
			return fmt.Errorf("library: 迁移到 %d 失败: %w", version+1, err)
		}
		// PRAGMA takes no parameters; the value is this package's own constant.
		if _, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, version+1)); err != nil {
			return err
		}
	}
	return nil
}

// rebuild rewrites the index from the disk. The index is a pure function of the
// folders, so there is nothing to merge: it is emptied and filled again.
func (l *Library) rebuild() {
	if l.db == nil {
		return
	}
	sources := l.scan()
	tx, err := l.db.Begin()
	if err != nil {
		log.Printf("library: 重建索引失败: %v", err)
		return
	}
	defer tx.Rollback()
	for _, statement := range []string{`DELETE FROM skills`, `DELETE FROM skill_sources`} {
		if _, err := tx.Exec(statement); err != nil {
			log.Printf("library: 重建索引失败: %v", err)
			return
		}
	}
	for _, source := range sources {
		installed := source.InstalledAt.Unix()
		result, err := tx.Exec(`INSERT INTO skill_sources(safe, installed_at) VALUES(?,?)`, source.Name, installed)
		if err != nil {
			log.Printf("library: 重建索引失败: %v", err)
			return
		}
		id, err := result.LastInsertId()
		if err != nil {
			log.Printf("library: 重建索引失败: %v", err)
			return
		}
		for _, skill := range source.Skills {
			if _, err := tx.Exec(`INSERT INTO skills(source_id, name, installed_at) VALUES(?,?,?)`, id, skill.Name, installed); err != nil {
				log.Printf("library: 重建索引失败: %v", err)
				return
			}
		}
	}
	if err := tx.Commit(); err != nil {
		log.Printf("library: 重建索引失败: %v", err)
	}
}

// indexed reads the sources back out of the index, in the order the tree shows
// them: by directory, and by skill name inside one.
func (l *Library) indexed() ([]domain.Source, error) {
	rows, err := l.db.Query(`
SELECT s.safe, s.installed_at, k.name
FROM skill_sources s LEFT JOIN skills k ON k.source_id = s.id
ORDER BY s.safe, k.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []domain.Source
	for rows.Next() {
		var (
			safe        string
			installedAt int64
			name        sql.NullString
		)
		if err := rows.Scan(&safe, &installedAt, &name); err != nil {
			return nil, err
		}
		if len(sources) == 0 || sources[len(sources)-1].Name != safe {
			sources = append(sources, domain.Source{
				Name:        safe,
				Repo:        domain.RepoOf(safe),
				InstalledAt: time.Unix(installedAt, 0),
				Skills:      []domain.Skill{},
			})
		}
		if name.Valid {
			last := &sources[len(sources)-1]
			last.Skills = append(last.Skills, domain.Skill{Name: name.String})
		}
	}
	return sources, rows.Err()
}
