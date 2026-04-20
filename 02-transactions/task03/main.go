// ============================================================
// Задача: Retry при serialization failure (код 40001)  🔴 Senior
// ============================================================
//
// При использовании SERIALIZABLE уровня изоляции Postgres может
// обнаружить конфликт между двумя транзакциями и откатить одну из них.
//
// Ошибка: pq: ERROR 40001 (serialization_failure)
//         "could not serialize access due to concurrent update"
//
// Это НЕ баг - это нормальное поведение.
// Приложение ОБЯЗАНО повторить транзакцию с начала.
//
// Аналогичный код - 40P01 (deadlock_detected):
//   Два процесса ждут блокировок друг друга → Postgres убивает одну.
//   Тоже нужен retry.
//
// pgx error codes:
//   pgerrcode.SerializationFailure = "40001"
//   pgerrcode.DeadlockDetected    = "40P01"
//
// Проверка в Go:
//   var pgErr *pgconn.PgError
//   if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.SerializationFailure {
//       // нужен retry
//   }
//
// Реализуй:
//
//   func IsRetryable(err error) bool
//     - возвращает true если код ошибки 40001 или 40P01
//
//   func RunWithRetry(ctx, pool, maxAttempts int, fn func(tx pgx.Tx) error) error
//     - выполняет fn в транзакции
//     - при IsRetryable ошибке: повторяет до maxAttempts раз
//     - при других ошибках: сразу возвращает
//     - Подсказка: BEGIN ISOLATION LEVEL SERIALIZABLE
//
// Запуск:
//   go mod tidy && go test -v ./...

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	pgSerializationFailure = "40001"
	pgDeadlockDetected     = "40P01"
)

func dsn() string {
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		return v
	}
	return "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
}

// TODO: реализуй IsRetryable - проверка что ошибка требует retry
func IsRetryable(err error) bool {
	// var pgErr *pgconn.PgError
	// if errors.As(err, &pgErr) {
	//     return pgErr.Code == pgSerializationFailure || pgErr.Code == pgDeadlockDetected
	// }
	// return false
	_ = pgConn(err)
	return false
}

func pgConn(err error) *pgconn.PgError {
	var pgErr *pgconn.PgError
	errors.As(err, &pgErr)
	return pgErr
}

// TODO: реализуй RunWithRetry - выполнение транзакции с повтором
func RunWithRetry(ctx context.Context, pool *pgxpool.Pool, maxAttempts int, fn func(tx pgx.Tx) error) error {
	// for attempt := 1; attempt <= maxAttempts; attempt++ {
	//     tx, err := pool.BeginTx(ctx, pgx.TxOptions{
	//         IsoLevel: pgx.Serializable,
	//     })
	//     if err != nil { return err }
	//
	//     err = fn(tx)
	//     if err != nil {
	//         tx.Rollback(ctx)
	//         if IsRetryable(err) && attempt < maxAttempts {
	//             continue  // повторяем
	//         }
	//         return err
	//     }
	//     if err := tx.Commit(ctx); err != nil {
	//         if IsRetryable(err) && attempt < maxAttempts {
	//             continue
	//         }
	//         return err
	//     }
	//     return nil
	// }
	// return fmt.Errorf("exceeded %d retry attempts", maxAttempts)
	return nil
}

func main() {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		log.Fatalf("connect: %v\nЗапусти: docker compose up -d", err)
	}
	defer pool.Close()

	// Инициализируем счётчик
	pool.Exec(ctx, "INSERT INTO counters (name, value) VALUES ('hits', 0) ON CONFLICT (name) DO UPDATE SET value = 0")

	// Запускаем конкурентные обновления с SERIALIZABLE
	// Некоторые получат 40001 → должны повториться
	attempts := 0
	err = RunWithRetry(ctx, pool, 5, func(tx pgx.Tx) error {
		attempts++
		var val int
		tx.QueryRow(ctx, "SELECT value FROM counters WHERE name = 'hits'").Scan(&val)
		_, err := tx.Exec(ctx, "UPDATE counters SET value = $1 WHERE name = 'hits'", val+1)
		return err
	})
	if err != nil {
		log.Fatalf("RunWithRetry: %v", err)
	}

	var final int
	pool.QueryRow(ctx, "SELECT value FROM counters WHERE name = 'hits'").Scan(&final)
	fmt.Printf("Выполнено за %d попыток, финальное значение: %d\n", attempts, final)

	// Cleanup
	pool.Exec(ctx, "DELETE FROM counters WHERE name = 'hits'")
}
