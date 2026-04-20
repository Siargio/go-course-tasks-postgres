// ============================================================
// Задача: Advisory Locks - координация воркеров без таблицы  🔴 Senior
// ============================================================
//
// Advisory Lock - пользовательская блокировка в PostgreSQL.
// Это не блокировка строки/таблицы, а именованный мьютекс на уровне БД.
//
// Зачем нужно:
//   - Гарантировать что cron-задача запускается только на одном сервере
//   - Лидер-выборы (leader election) в кластере
//   - Координация воркеров без отдельной таблицы блокировок
//
// Функции:
//   pg_try_advisory_lock(key bigint) → bool
//     Попытаться захватить блокировку. Не ждёт - сразу возвращает.
//     true  → блокировка захвачена
//     false → кто-то другой уже держит её
//
//   pg_advisory_unlock(key bigint) → bool
//     Освободить блокировку.
//
//   pg_advisory_lock(key bigint)
//     Захватить блокировку, ждать если занята (BLOCKING).
//
// Блокировка привязана к СОЕДИНЕНИЮ (не транзакции).
// Поэтому нужно получить конкретное соединение из пула: pool.Acquire()
//
//   conn, err := pool.Acquire(ctx)
//   defer conn.Release()
//   conn.Exec(ctx, "SELECT pg_advisory_lock($1)", key)
//   // ... работа ...
//   conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", key)
//
// Реализуй:
//
//   func TryLock(ctx, conn *pgxpool.Conn, key int64) (bool, error)
//     - SELECT pg_try_advisory_lock($1)
//
//   func Unlock(ctx, conn *pgxpool.Conn, key int64) error
//     - SELECT pg_advisory_unlock($1)
//
//   func WithAdvisoryLock(ctx, pool, key int64, fn func() error) error
//     - Acquire соединение из пула
//     - TryLock → если false → return ErrLockBusy
//     - defer Unlock
//     - fn()
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
	"sync"
	"sync/atomic"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrLockBusy = errors.New("advisory lock is already held by another connection")

func dsn() string {
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		return v
	}
	return "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
}

// TODO: реализуй TryLock - pg_try_advisory_lock
func TryLock(ctx context.Context, conn *pgxpool.Conn, key int64) (bool, error) {
	// var locked bool
	// err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&locked)
	// return locked, err
	return false, nil
}

// TODO: реализуй Unlock - pg_advisory_unlock
func Unlock(ctx context.Context, conn *pgxpool.Conn, key int64) error {
	// _, err := conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", key)
	// return err
	return nil
}

// TODO: реализуй WithAdvisoryLock - захватить, выполнить, освободить
func WithAdvisoryLock(ctx context.Context, pool *pgxpool.Pool, key int64, fn func() error) error {
	// conn, err := pool.Acquire(ctx)
	// if err != nil { return err }
	// defer conn.Release()
	//
	// locked, err := TryLock(ctx, conn, key)
	// if err != nil { return err }
	// if !locked { return ErrLockBusy }
	//
	// defer Unlock(ctx, conn, key)
	// return fn()
	return nil
}

func main() {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	const lockKey = int64(42_000_000)

	// Запускаем 5 воркеров - только 1 должен захватить блокировку
	var (
		wg         sync.WaitGroup
		executions atomic.Int32
		busy       atomic.Int32
	)

	for i := range 5 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := WithAdvisoryLock(ctx, pool, lockKey, func() error {
				executions.Add(1)
				fmt.Printf("  Воркер %d: выполняет критическую секцию\n", id)
				return nil
			})
			if errors.Is(err, ErrLockBusy) {
				busy.Add(1)
				fmt.Printf("  Воркер %d: блокировка занята, пропускаем\n", id)
			} else if err != nil {
				log.Printf("  Воркер %d: ошибка: %v", id, err)
			}
		}(i)
	}

	wg.Wait()
	fmt.Printf("\nВыполнений: %d, Заблокировано: %d\n", executions.Load(), busy.Load())

	if executions.Load() > 1 {
		fmt.Println("ВНИМАНИЕ: Advisory Lock не работает - несколько воркеров выполнились одновременно!")
	} else {
		fmt.Println("Advisory Lock работает - только один воркер захватил блокировку")
	}
}
