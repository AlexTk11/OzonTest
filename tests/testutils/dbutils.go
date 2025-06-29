package testutils

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

// SetupTestDB создает и настраивает тестовую базу данных
func SetupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbHost := getEnv("TEST_DB_HOST", "localhost")
	dbPort := getEnv("TEST_DB_PORT", "5433")
	dbUser := getEnv("TEST_DB_USER", "testuser")
	dbPassword := getEnv("TEST_DB_PASSWORD", "testpass")
	dbName := getEnv("TEST_DB_NAME", "test_comments_db")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Skipf("Failed to connect to test database: %v", err)
	}

	if err := db.Ping(); err != nil {
		t.Skipf("Failed to ping test database: %v", err)
	}

	createTestTables(t, db)
	return db
}

// CleanTestDB очищает тестовую БД
func CleanTestDB(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec("TRUNCATE TABLE posts CASCADE")
	if err != nil {
		t.Logf("Warning: failed to truncate tables: %v", err)
	}
}

// createTestTables создание таблиц для тестов
func createTestTables(t *testing.T, db *sql.DB) {
	t.Helper()

	// Удаляем таблицы если существуют
	_, err := db.Exec("DROP TABLE IF EXISTS comments CASCADE")
	if err != nil {
		t.Fatalf("Failed to drop comments table: %v", err)
	}

	_, err = db.Exec("DROP TABLE IF EXISTS posts CASCADE")
	if err != nil {
		t.Fatalf("Failed to drop posts table: %v", err)
	}

	// Создаем таблицу постов
	createPostsTable := `
        CREATE TABLE posts (
            id VARCHAR(36) PRIMARY KEY,
            text TEXT NOT NULL,
            comments_enabled BOOLEAN NOT NULL DEFAULT true,
            created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
        )`

	_, err = db.Exec(createPostsTable)
	if err != nil {
		t.Fatalf("Failed to create posts table: %v", err)
	}

	// Создаем таблицу комментариев
	createCommentsTable := `
        CREATE TABLE comments (
            id VARCHAR(36) PRIMARY KEY,
            post_id VARCHAR(36) NOT NULL,
            parent_id VARCHAR(36),
            text TEXT NOT NULL,
            created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
            FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
            FOREIGN KEY (parent_id) REFERENCES comments(id) ON DELETE CASCADE
        )`

	_, err = db.Exec(createCommentsTable)
	if err != nil {
		t.Fatalf("Failed to create comments table: %v", err)
	}

	// Создаем индексы
	indexes := []string{
		"CREATE INDEX idx_comments_post_id ON comments(post_id)",
		"CREATE INDEX idx_comments_parent_id ON comments(parent_id)",
		"CREATE INDEX idx_comments_created_at ON comments(created_at)",
	}

	for _, index := range indexes {
		_, err = db.Exec(index)
		if err != nil {
			t.Logf("Warning: failed to create index: %v", err)
		}
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
