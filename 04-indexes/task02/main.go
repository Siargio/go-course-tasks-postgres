// ============================================================
// Задача: GIN для JSONB и EXPLAIN - выбор стратегии  🔴 Senior
// ============================================================
//
// Типы индексов PostgreSQL (кроме B-tree):
//
//   GIN (Generalized Inverted Index):
//     Подходит для: JSONB, arrays, tsvector (full-text search)
//     Операторы: @>, <@, ?, ?|, ?&, @@
//     Пример: metadata @> '{"tag": "urgent"}'
//     Создание: CREATE INDEX idx_orders_meta ON orders USING GIN(metadata)
//
//   GiST (Generalized Search Tree):
//     Подходит для: геоданные, диапазоны (tsrange, numrange), фигуры
//     Пример: valid_range && '[2024-01-01, 2024-12-31]'::daterange
//
//   BRIN (Block Range Index):
//     Очень маленький индекс для ОГРОМНЫХ таблиц с монотонно возрастающими данными
//     Пример: created_at для append-only логов
//     В 100-1000x меньше B-tree, но не так точен
//
//   Hash:
//     Только для равенства (=). Быстрее B-tree для точного совпадения,
//     но не поддерживает сортировку и диапазоны.
//
// Реализуй:
//
//   func CreateGINIndex(ctx, pool) error
//     - CREATE INDEX IF NOT EXISTS idx_orders_metadata ON orders USING GIN(metadata)
//
//   func SearchByMetadata(ctx, pool, jsonFilter string) ([]int64, error)
//     - SELECT id FROM orders WHERE metadata @> $1::jsonb
//     - jsonFilter: например '{"status":"vip","tag":"urgent"}'
//
//   func InsertOrderWithMeta(ctx, pool, userID int, metadata string) (int64, error)
//     - INSERT INTO orders (user_id, status, amount, metadata) VALUES (...)
//     - metadata - JSONB строка
//
//   func ExplainSearchByMeta(ctx, pool, jsonFilter string) (string, error)
//     - EXPLAIN (ANALYZE, FORMAT TEXT) SELECT id FROM orders WHERE metadata @> $1::jsonb
//
// Запуск:
//   go mod tidy && go test -v ./...

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func dsn() string {
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		return v
	}
	return "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
}

// TODO: реализуй CreateGINIndex
func CreateGINIndex(ctx context.Context, pool *pgxpool.Pool) error {
	// _, err := pool.Exec(ctx,
	//     "CREATE INDEX IF NOT EXISTS idx_orders_metadata ON orders USING GIN(metadata)")
	// return err
	return nil
}

// TODO: реализуй SearchByMetadata - JSONB containment @>
func SearchByMetadata(ctx context.Context, pool *pgxpool.Pool, jsonFilter string) ([]int64, error) {
	// rows, err := pool.Query(ctx,
	//     "SELECT id FROM orders WHERE metadata @> $1::jsonb", jsonFilter)
	// if err != nil { return nil, err }
	// defer rows.Close()
	// var ids []int64
	// for rows.Next() {
	//     var id int64
	//     rows.Scan(&id)
	//     ids = append(ids, id)
	// }
	// return ids, rows.Err()
	return nil, nil
}

// TODO: реализуй InsertOrderWithMeta
func InsertOrderWithMeta(ctx context.Context, pool *pgxpool.Pool, userID int, metadata string) (int64, error) {
	// var id int64
	// err := pool.QueryRow(ctx,
	//     "INSERT INTO orders (user_id, status, amount, metadata) VALUES ($1, 'pending', 0, $2::jsonb) RETURNING id",
	//     userID, metadata,
	// ).Scan(&id)
	// return id, err
	return 0, nil
}

// TODO: реализуй ExplainSearchByMeta
func ExplainSearchByMeta(ctx context.Context, pool *pgxpool.Pool, jsonFilter string) (string, error) {
	// rows, err := pool.Query(ctx,
	//     "EXPLAIN (ANALYZE, FORMAT TEXT) SELECT id FROM orders WHERE metadata @> $1::jsonb",
	//     jsonFilter,
	// )
	// ...
	_ = strings.Join
	return "", nil
}

func main() {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	// Вставляем тестовые заказы с JSONB метаданными
	metas := []string{
		`{"tag": "urgent", "region": "EU"}`,
		`{"tag": "normal", "region": "US"}`,
		`{"tag": "urgent", "region": "US"}`,
		`{"tag": "vip", "region": "EU"}`,
	}

	var insertedIDs []int64
	for i, meta := range metas {
		id, err := InsertOrderWithMeta(ctx, pool, i+1, meta)
		if err != nil {
			log.Fatalf("InsertOrderWithMeta: %v", err)
		}
		insertedIDs = append(insertedIDs, id)
	}
	defer func() {
		for _, id := range insertedIDs {
			pool.Exec(ctx, "DELETE FROM orders WHERE id = $1", id)
		}
		pool.Exec(ctx, "DROP INDEX IF EXISTS idx_orders_metadata")
	}()

	filter := `{"tag": "urgent"}`

	// Без индекса
	fmt.Println("=== БЕЗ GIN индекса ===")
	plan, _ := ExplainSearchByMeta(ctx, pool, filter)
	fmt.Println(plan)

	// Создаём GIN индекс
	if err := CreateGINIndex(ctx, pool); err != nil {
		log.Fatalf("CreateGINIndex: %v", err)
	}

	pool.Exec(ctx, "ANALYZE orders")

	// С индексом
	fmt.Println("\n=== С GIN индексом ===")
	plan, _ = ExplainSearchByMeta(ctx, pool, filter)
	fmt.Println(plan)

	// Поиск
	ids, _ := SearchByMetadata(ctx, pool, filter)
	fmt.Printf("\nНайдено заказов с tag='urgent': %d (ожидаем 2)\n", len(ids))
}
