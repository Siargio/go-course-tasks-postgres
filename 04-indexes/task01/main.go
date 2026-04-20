// ============================================================
// Задача: B-tree и Partial Index - EXPLAIN ANALYZE  🟡 Middle
// ============================================================
//
// Индекс - структура данных, ускоряющая поиск ценой дополнительного места
// и замедления INSERT/UPDATE/DELETE.
//
// B-tree (по умолчанию):
//   Подходит для: =, <, >, BETWEEN, LIKE 'prefix%', ORDER BY
//   Создаётся: CREATE INDEX idx_orders_user ON orders(user_id)
//   Не помогает при: LIKE '%suffix', полнотекстовый поиск, NULL
//
// Partial Index (частичный):
//   Индексирует только строки удовлетворяющие WHERE.
//   Меньший размер → быстрее, меньше памяти.
//   Пример: индекс только на "pending" заказы
//     CREATE INDEX idx_orders_pending ON orders(user_id) WHERE status = 'pending'
//
// EXPLAIN ANALYZE:
//   EXPLAIN        - план запроса (без выполнения)
//   EXPLAIN ANALYZE - план + реальное выполнение + время
//
//   Seq Scan  - полный перебор (медленно при большом объёме)
//   Index Scan - использует индекс (быстро)
//   Bitmap Index Scan → Bitmap Heap Scan - для больших результатов
//
// Реализуй:
//
//   func CreateIndexes(ctx, pool) error
//     - CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id)
//     - CREATE INDEX IF NOT EXISTS idx_orders_pending ON orders(user_id, created_at)
//         WHERE status = 'pending'
//
//   func ExplainQuery(ctx, pool, query string, args ...any) (string, error)
//     - EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) <query>
//     - собирает все строки вывода в одну строку
//
//   func SeedOrders(ctx, pool, n int) error
//     - INSERT n заказов с рандомными user_id (1..100) и статусами
//
// Запуск:
//   go mod tidy && go test -v ./...

package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func dsn() string {
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		return v
	}
	return "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
}

// TODO: реализуй CreateIndexes
func CreateIndexes(ctx context.Context, pool *pgxpool.Pool) error {
	// _, err := pool.Exec(ctx, `
	//     CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);
	// `)
	// if err != nil { return err }
	//
	// _, err = pool.Exec(ctx, `
	//     CREATE INDEX IF NOT EXISTS idx_orders_pending
	//     ON orders(user_id, created_at)
	//     WHERE status = 'pending'
	// `)
	// return err
	return nil
}

// TODO: реализуй ExplainQuery - EXPLAIN ANALYZE
func ExplainQuery(ctx context.Context, pool *pgxpool.Pool, query string, args ...any) (string, error) {
	// explainSQL := "EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) " + query
	// rows, err := pool.Query(ctx, explainSQL, args...)
	// if err != nil { return "", err }
	// defer rows.Close()
	//
	// var lines []string
	// for rows.Next() {
	//     var line string
	//     rows.Scan(&line)
	//     lines = append(lines, line)
	// }
	// return strings.Join(lines, "\n"), rows.Err()
	_ = strings.Join
	return "", nil
}

// TODO: реализуй SeedOrders - заполнить таблицу тестовыми данными
func SeedOrders(ctx context.Context, pool *pgxpool.Pool, n int) error {
	statuses := []string{"pending", "processing", "completed", "failed"}
	// batch := &pgx.Batch{}
	for i := range n {
		userID := rand.Intn(100) + 1
		status := statuses[rand.Intn(len(statuses))]
		amount := float64(rand.Intn(10000)) / 100.0
		// batch.Queue("INSERT INTO orders (user_id, status, amount) VALUES ($1, $2, $3)", userID, status, amount)
		_ = i
		_ = userID
		_ = status
		_ = amount
	}
	// br := pool.SendBatch(ctx, batch)
	// return br.Close()
	return nil
}

func main() {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	fmt.Println("Заполняем 10000 заказов...")
	if err := SeedOrders(ctx, pool, 10000); err != nil {
		log.Fatalf("SeedOrders: %v", err)
	}

	// Запрос БЕЗ индекса
	fmt.Println("\n--- БЕЗ индекса ---")
	plan, _ := ExplainQuery(ctx, pool, "SELECT * FROM orders WHERE user_id = $1", 42)
	fmt.Println(plan)

	// Создаём индексы
	fmt.Println("\nСоздаём индексы...")
	if err := CreateIndexes(ctx, pool); err != nil {
		log.Fatalf("CreateIndexes: %v", err)
	}

	// Запрос С индексом
	fmt.Println("\n--- С индексом idx_orders_user_id ---")
	plan, _ = ExplainQuery(ctx, pool, "SELECT * FROM orders WHERE user_id = $1", 42)
	fmt.Println(plan)

	// Запрос с partial index
	fmt.Println("\n--- С partial index (только pending) ---")
	start := time.Now()
	plan, _ = ExplainQuery(ctx, pool,
		"SELECT * FROM orders WHERE user_id = $1 AND status = 'pending' ORDER BY created_at DESC", 42)
	fmt.Printf("Время: %v\n%s\n", time.Since(start), plan)

	// Cleanup
	pool.Exec(ctx, "DELETE FROM orders WHERE id > 0")
	pool.Exec(ctx, "DROP INDEX IF EXISTS idx_orders_user_id")
	pool.Exec(ctx, "DROP INDEX IF EXISTS idx_orders_pending")
}
