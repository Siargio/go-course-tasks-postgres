// ============================================================
// Задача: Подключение к PostgreSQL через pgxpool  🟢 Junior
// ============================================================
//
// Почему pgx, а не database/sql?
//
//   database/sql - стандартный интерфейс Go для любых БД.
//   pgx          - нативный PostgreSQL драйвер, написанный специально для PG.
//
//   Преимущества pgx перед database/sql:
//     - Нативные типы PG: arrays, JSONB, UUID, INET без маршалинга строк
//     - Лучшая производительность: бинарный протокол, меньше аллокаций
//     - pgxpool: встроенный пул соединений с min/max/idle настройками
//     - Поддержка LISTEN/NOTIFY, COPY, prepared statements
//
// Пул соединений (pgxpool):
//
//   Создание TCP-соединения дорого (handshake, auth, SSL).
//   Пул держит N открытых соединений и переиспользует их.
//
//   MaxConns     - максимум одновременных соединений
//   MinConns     - минимум "тёплых" соединений (всегда готовы)
//   MaxConnLifetime - максимальное время жизни соединения (защита от утечек)
//   MaxConnIdleTime - закрыть соединение если простаивает N минут
//
// DSN (Data Source Name) формат:
//   postgres://user:password@host:port/dbname?sslmode=disable
//   host=localhost port=5432 user=postgres password=postgres dbname=postgres
//
// Реализуй:
//
//   func Connect(ctx, dsn string) (*pgxpool.Pool, error)
//     - создаёт пул через pgxpool.New
//     - проверяет соединение через pool.Ping
//
//   func ConnectWithConfig(ctx, dsn string, maxConns int32) (*pgxpool.Pool, error)
//     - pgxpool.ParseConfig + устанавливает MaxConns
//     - создаёт пул через pgxpool.NewWithConfig
//
//   func DSN() string
//     - читает POSTGRES_DSN из env, иначе дефолтный DSN для docker-compose
//
// Запуск Postgres:
//   docker compose up -d
//
// Запуск тестов:
//   go mod tidy && go test -v ./...

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TODO: реализуй DSN - читает POSTGRES_DSN из env или возвращает дефолт
func DSN() string {
	if dsn := os.Getenv("POSTGRES_DSN"); dsn != "" {
		return dsn
	}
	// Дефолтные значения совпадают с docker-compose.yml
	return "postgres://postgres:postgres@localhost:5433/postgres?sslmode=disable"
}

// TODO: реализуй Connect - создаёт пул соединений
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	// Подсказка:
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

// TODO: реализуй ConnectWithConfig - пул с кастомными настройками
func ConnectWithConfig(ctx context.Context, dsn string, maxConns int32) (*pgxpool.Pool, error) {
	// Подсказка:
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = maxConns
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	return pgxpool.NewWithConfig(ctx, cfg)
}

func main() {
	ctx := context.Background()
	dsn := DSN()

	fmt.Printf("Подключаемся к: %s\n", dsn)

	pool, err := Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("Connect: %v\n"+
			"Убедись что Postgres запущен: docker compose up -d", err)
	}
	defer pool.Close()

	// Статистика пула
	stats := pool.Stat()
	fmt.Printf("Пул создан:\n")
	fmt.Printf("  TotalConns:    %d\n", stats.TotalConns())
	fmt.Printf("  IdleConns:     %d\n", stats.IdleConns())
	fmt.Printf("  MaxConns:      %d\n", stats.MaxConns())

	// Версия PostgreSQL
	var version string
	err = pool.QueryRow(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		log.Fatalf("QueryRow: %v", err)
	}
	fmt.Printf("\nPostgreSQL: %s\n", version)

	// Кастомный пул
	pool2, err := ConnectWithConfig(ctx, dsn, 10)
	if err != nil {
		log.Fatalf("ConnectWithConfig: %v", err)
	}
	defer pool2.Close()
	fmt.Printf("\nПул с MaxConns=10: OK (MaxConns=%d)\n", pool2.Stat().MaxConns())
}
