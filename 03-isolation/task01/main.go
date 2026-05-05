// ============================================================
// Задача: Non-repeatable Read - Read Committed vs Repeatable Read  🟡 Middle
// ============================================================
//
// Non-repeatable Read - аномалия при которой одна транзакция дважды
// читает одну строку и получает РАЗНЫЕ значения.
//
// Воспроизведение:
//
//   Транзакция A (READ COMMITTED):
//     t=1: SELECT value FROM counters WHERE name='x'  → 100
//     t=3: SELECT value FROM counters WHERE name='x'  → 200  ← другой результат!
//
//   Транзакция B:
//     t=2: UPDATE counters SET value=200 WHERE name='x'; COMMIT;
//
//   Проблема: A видит изменение, закоммиченное B между двумя SELECT.
//   При READ COMMITTED это нормальное поведение (каждый SELECT видит
//   актуальный снимок данных).
//
// Решение - REPEATABLE READ:
//   Вся транзакция работает с одним снимком данных (snapshot).
//   Повторный SELECT вернёт то же значение, что и первый.
//
// Реализуй:
//
//   func ReadTwiceReadCommitted(ctx, pool, name string, updateBetween func()) (first, second int, err error)
//     - BEGIN ISOLATION LEVEL READ COMMITTED
//     - SELECT value (first)
//     - вызвать updateBetween() - симулирует обновление из другой транзакции
//     - SELECT value (second) - в READ COMMITTED увидит новое значение
//     - ROLLBACK (только читаем)
//
//   func ReadTwiceRepeatableRead(ctx, pool, name string, updateBetween func()) (first, second int, err error)
//     - BEGIN ISOLATION LEVEL REPEATABLE READ
//     - то же самое, но second == first (снимок не меняется)
//
// Запуск:
//   go mod tidy && go test -v ./...

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func dsn() string {
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		return v
	}
	return "postgres://dev:dev@localhost:5433/devdb?sslmode=disable"
}

// TODO: реализуй ReadTwiceReadCommitted
func ReadTwiceReadCommitted(ctx context.Context, pool *pgxpool.Pool, name string, updateBetween func()) (first, second int, err error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)

	tx.QueryRow(ctx, "SELECT value FROM counters WHERE name = $1", name).Scan(&first)
	updateBetween() // другая горутина/транзакция меняет значение
	tx.QueryRow(ctx, "SELECT value FROM counters WHERE name = $1", name).Scan(&second)
	// При READ COMMITTED: first != second (видим новое закоммиченное значение)
	return first, second, nil
}

// TODO: реализуй ReadTwiceRepeatableRead
func ReadTwiceRepeatableRead(ctx context.Context, pool *pgxpool.Pool, name string, updateBetween func()) (first, second int, err error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)

	tx.QueryRow(ctx, "SELECT value FROM counters WHERE name = $1", name).Scan(&first)
	updateBetween()
	tx.QueryRow(ctx, "SELECT value FROM counters WHERE name = $1", name).Scan(&second)
	// При REPEATABLE READ: first == second (снимок зафиксирован в BEGIN)
	return first, second, nil
}

func main() {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	// Подготовка
	name := "iso-test"
	pool.Exec(ctx, "INSERT INTO counters (name, value) VALUES ($1, 100) ON CONFLICT (name) DO UPDATE SET value = 100", name)
	defer pool.Exec(ctx, "DELETE FROM counters WHERE name = $1", name)

	// updateBetween: коммитит изменение из "другой транзакции"
	update := func() {
		pool.Exec(ctx, "UPDATE counters SET value = 200 WHERE name = $1", name)
	}

	// Подготовка: вставляем тестовые данные
	_, err = pool.Exec(ctx, `
    INSERT INTO counters (name, value) VALUES ('iso-test', 100) 
    ON CONFLICT (name) DO UPDATE SET value = 100
`)
	if err != nil {
		log.Fatalf("insert failed: %v", err)
	}
	defer pool.Exec(ctx, "DELETE FROM counters WHERE name = 'iso-test'")

	// READ COMMITTED - non-repeatable read
	fmt.Println("=== READ COMMITTED ===")
	pool.Exec(ctx, "UPDATE counters SET value = 100 WHERE name = $1", name)
	f1, s1, _ := ReadTwiceReadCommitted(ctx, pool, name, update)
	fmt.Printf("Первый SELECT: %d, Второй SELECT: %d\n", f1, s1)
	if f1 != s1 {
		fmt.Println("Non-repeatable read: значения РАЗНЫЕ (ожидаемо при READ COMMITTED)")
	}

	// REPEATABLE READ - защита от non-repeatable read
	fmt.Println("\n=== REPEATABLE READ ===")
	pool.Exec(ctx, "UPDATE counters SET value = 100 WHERE name = $1", name)
	f2, s2, _ := ReadTwiceRepeatableRead(ctx, pool, name, update)
	fmt.Printf("Первый SELECT: %d, Второй SELECT: %d\n", f2, s2)
	if f2 == s2 {
		fmt.Println("Repeatable read: значения ОДИНАКОВЫЕ (защита работает)")
	}
}
