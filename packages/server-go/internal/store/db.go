package store

import (
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Store struct {
	db *gorm.DB
}

type OpenOptions struct {
	MaxOpenConns int
	TxLock       string
}

func Open(dsn string) (*Store, error) {
	return OpenWithOptions(dsn, OpenOptions{})
}

func OpenWithOptions(dsn string, opts OpenOptions) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(sqliteDSNWithOptions(dsn, opts)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if dsn == ":memory:" {
		// :memory: requires single conn — sqlite creates a fresh DB per
		// connection, so a pool of >1 conn would see N independent DBs.
		sqlDB.SetMaxOpenConns(1)
	} else if opts.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(opts.MaxOpenConns)
	}

	// PERF: WAL only meaningful on file-backed sqlite. :memory: WAL is no-op
	// at best, contention source at worst (per-test isolated in-mem DB pays
	// pragma cost N times for nothing). Skip for :memory:; file dsn keeps WAL.
	pragmas := []string{
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	if dsn != ":memory:" {
		pragmas = append([]string{"PRAGMA journal_mode=WAL"}, pragmas...)
	}
	for _, pragma := range pragmas {
		if _, err := sqlDB.Exec(pragma); err != nil {
			return nil, err
		}
	}

	return &Store{db: db}, nil
}

func sqliteDSNWithPragmas(dsn string) string {
	return sqliteDSNWithOptions(dsn, OpenOptions{})
}

func sqliteDSNWithOptions(dsn string, opts OpenOptions) string {
	if dsn == ":memory:" {
		return dsn
	}

	params := []string{
		"_busy_timeout=5000",
		"_foreign_keys=on",
		"_journal_mode=WAL",
	}
	if opts.TxLock != "" {
		params = append(params, "_txlock="+opts.TxLock)
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + strings.Join(params, "&")
}

func (s *Store) DB() *gorm.DB {
	return s.db
}

func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
