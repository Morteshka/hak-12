// Package db — подключение к PostgreSQL и применение SQL-миграций.
package db

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"invoicekeeper/migrations"
)

// Connect подключается к БД с ретраями (БД в docker может стартовать дольше приложения).
func Connect(dsn string) (*gorm.DB, error) {
	var lastErr error
	for i := 0; i < 30; i++ {
		g, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.New(log.New(os.Stdout, "", log.LstdFlags), logger.Config{
				SlowThreshold: 500 * time.Millisecond, LogLevel: logger.Warn, IgnoreRecordNotFoundError: true,
			}),
			TranslateError: true,
		})
		if err == nil {
			var sqlDB *sql.DB
			if sqlDB, err = g.DB(); err == nil {
				if err = sqlDB.Ping(); err == nil {
					return g, nil
				}
			}
		}
		lastErr = err
		log.Printf("db: waiting for database (%d/30): %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("database unavailable: %w", lastErr)
}

// Migrate применяет ещё не применённые *.sql по порядку имён. Каждая — в своей транзакции.
func Migrate(g *gorm.DB) error {
	if err := g.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`).Error; err != nil {
		return err
	}
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && len(e.Name()) > 4 && e.Name()[len(e.Name())-4:] == ".sql" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var n int64
		g.Raw(`SELECT count(*) FROM schema_migrations WHERE version = ?`, name).Scan(&n)
		if n > 0 {
			continue
		}
		sqlBytes, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			return err
		}
		err = g.Transaction(func(tx *gorm.DB) error {
			// выполняем по одному выражению: надёжно для любого режима драйвера
			for _, stmt := range strings.Split(string(sqlBytes), ";") {
				if strings.TrimSpace(stmt) == "" {
					continue
				}
				if err := tx.Exec(stmt).Error; err != nil {
					return err
				}
			}
			return tx.Exec(`INSERT INTO schema_migrations(version) VALUES (?)`, name).Error
		})
		if err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
		log.Printf("db: applied migration %s", name)
	}
	return nil
}
