package postgres

import (
	"context"
	"embed"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"high-perf-wallet/pkg/logger"
	"sort"
	"strings"
)

// Menggunakan directive //go:embed untuk menyisipkan berkas SQL secara otomatis saat kompilasi
//go:embed migrations/*.sql
var migrationFiles embed.FS

func RunMigrations(ctx context.Context, dbPool *pgxpool.Pool) error {
	// 1. Buat tabel schema_migrations jika belum ada untuk mencatat riwayat migrasi yang sudah jalan
	_, err := dbPool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migration table: %w", err)
	}

	// 2. Baca daftar file SQL dari embed.FS
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var filenames []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			filenames = append(filenames, entry.Name())
		}
	}
	// Urutkan nama file secara alfabetis (000001, 000002, dst)
	sort.Strings(filenames)

	// 3. Eksekusi setiap file SQL secara berurutan
	for _, filename := range filenames {
		var exists bool
		err = dbPool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", filename).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check migration status for %s: %w", filename, err)
		}

		// Jika file ini sudah pernah dieksekusi sebelumnya, skip/lewati
		if exists {
			continue
		}

		logger.Log.Info("Running database migration...", zap.String("file", filename))

		// Baca isi file SQL
		content, err := migrationFiles.ReadFile("migrations/" + filename)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", filename, err)
		}

		// Mulai transaksi database untuk mengamankan jalannya migrasi (jika gagal, dibatalkan total)
		tx, err := dbPool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer tx.Rollback(ctx)

		// Jalankan query SQL dari file
		_, err = tx.Exec(ctx, string(content))
		if err != nil {
			return fmt.Errorf("failed to execute migration query in %s: %w", filename, err)
		}

		// Catat riwayat migrasi yang sukses
		_, err = tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", filename)
		if err != nil {
			return fmt.Errorf("failed to insert migration version for %s: %w", filename, err)
		}

		// Commit transaksi
		err = tx.Commit(ctx)
		if err != nil {
			return fmt.Errorf("failed to commit migration transaction: %w", err)
		}

		logger.Log.Info("Migration applied successfully", zap.String("file", filename))
	}

	return nil
}
