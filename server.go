package main

import (
	"PostAndComment/graph"
	"PostAndComment/storage"
	"PostAndComment/storage/memory"
	"PostAndComment/storage/postgres"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/websocket"
	_ "github.com/lib/pq"
	"github.com/vektah/gqlparser/v2/ast"
)

const defaultPort = "8080"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	// извлекаем тип хранилища из переменной STORAGE_TYPE
	storageType := os.Getenv("STORAGE_TYPE")
	if storageType == "" {
		storageType = "memory"
	}

	var storageInstance storage.Storage
	var err error

	switch storageType {
	case "postgres":
		storageInstance, err = initPostgresStorage()
		if err != nil {
			log.Fatalf("Failed to initialize Postgres storage: %v", err)
		}
		log.Println("Using Postgres storage")
	case "memory":
		storageInstance = memory.New()
		log.Println("Using in-memory storage")
	default:
		log.Fatalf("Unknown storage type: %s, use 'postgres' or 'memory'", storageType)
	}

	srv := handler.New(graph.NewExecutableSchema(graph.Config{
		Resolvers: &graph.Resolver{Storage: storageInstance}}))

	srv.AddTransport(transport.Websocket{
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		KeepAlivePingInterval: 5,
	})

	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})

	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))

	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

	http.Handle("/", playground.Handler("GraphQL playground", "/query"))
	http.Handle("/query", srv)

	log.Printf("Server running on http://localhost:%s/", port)
	log.Printf("GraphQL playground available at http://localhost:%s/", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func initPostgresStorage() (storage.Storage, error) {

	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "postgres")
	dbname := getEnv("DB_NAME", "comments_db")
	sslmode := getEnv("DB_SSLMODE", "disable")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)

	var db *sql.DB
	var err error

	maxRetries := 30 //кол-во попыток подключения
	for i := 0; i < maxRetries; i++ {
		db, err = sql.Open("postgres", connStr) //Попытка подключиться
		if err != nil {
			log.Printf("Failed to open database connection, attempt %d/%d : %v", i+1, maxRetries, err)
			time.Sleep(2 * time.Second)
			continue
		}

		err = db.Ping() // Попытка пинга
		if err != nil {
			log.Printf("Failed to ping database, attempt %d/%d : %v", i+1, maxRetries, err)
			db.Close()
			time.Sleep(2 * time.Second)
			continue
		}

		break
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database after %d attempts: %v", maxRetries, err)
	}

	db.SetMaxOpenConns(25)                 // Кол-во соединений
	db.SetMaxIdleConns(5)                  // Кол-во готовых к подключению соединений
	db.SetConnMaxLifetime(5 * time.Minute) // Время жизни соединения

	if err := createTables(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}

	log.Println("Successfully connected to Postgres storage")
	return postgres.New(db), nil
}

func createTables(db *sql.DB) error {
	// Создание таблицы постов
	createPostsTable := `
        CREATE TABLE IF NOT EXISTS posts (
            id VARCHAR(36) PRIMARY KEY,
            text TEXT NOT NULL,
            comments_enabled BOOLEAN NOT NULL DEFAULT true,
            created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
        );
    `

	// Создание таблицы комментариев
	createCommentsTable := `
        CREATE TABLE IF NOT EXISTS comments (
            id VARCHAR(36) PRIMARY KEY,
            post_id VARCHAR(36) NOT NULL,
            parent_id VARCHAR(36),
            text TEXT NOT NULL,
            created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
            FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
            FOREIGN KEY (parent_id) REFERENCES comments(id) ON DELETE CASCADE
        );
    `

	// Создание индексов
	createIndexes := `
        CREATE INDEX IF NOT EXISTS idx_comments_post_id ON comments(post_id);
        CREATE INDEX IF NOT EXISTS idx_comments_parent_id ON comments(parent_id);
        CREATE INDEX IF NOT EXISTS idx_comments_created_at ON comments(created_at);
        CREATE INDEX IF NOT EXISTS idx_posts_created_at ON posts(created_at);
    `

	queries := []string{createPostsTable, createCommentsTable, createIndexes}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to create tables: %v", err)
		}
	}

	log.Println("Database tables created successfully")
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
