package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Skipf("PostgreSQL недоступен: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("ping: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func setupDoctors(t *testing.T, pool *pgxpool.Pool) (alice, bob string) {
	t.Helper()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	alice = "Alice-" + suffix
	bob = "Bob-" + suffix
	ctx := context.Background()
	pool.Exec(ctx, "INSERT INTO on_call (doctor, active) VALUES ($1, true), ($2, true)", alice, bob)
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM on_call WHERE doctor IN ($1, $2)", alice, bob)
	})
	return
}

func activeCount(t *testing.T, pool *pgxpool.Pool, doctors ...string) int {
	t.Helper()
	var count int
	pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM on_call WHERE active = true AND doctor = ANY($1)",
		doctors,
	).Scan(&count)
	return count
}

func TestTryGoOffCallSuccess(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	alice, bob := setupDoctors(t, pool)

	// Alice может уйти - Bob остаётся
	if err := TryGoOffCall(ctx, pool, alice, pgx.Serializable); err != nil {
		t.Fatalf("TryGoOffCall(Alice): %v", err)
	}

	if n := activeCount(t, pool, alice, bob); n != 1 {
		t.Errorf("activeCount = %d, хотим 1", n)
	}
}

func TestTryGoOffCallLastDoctor(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	solo := "Solo-" + suffix
	pool.Exec(ctx, "INSERT INTO on_call (doctor, active) VALUES ($1, true)", solo)
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM on_call WHERE doctor = $1", solo) })

	err := TryGoOffCall(ctx, pool, solo, pgx.Serializable)
	if !errors.Is(err, ErrLastDoctor) {
		t.Errorf("ожидали ErrLastDoctor, получили: %v", err)
	}

	// Остался активным
	if n := activeCount(t, pool, solo); n != 1 {
		t.Errorf("после ErrLastDoctor доктор должен остаться активным, activeCount=%d", n)
	}
}

func TestSerializablePreventsWriteSkew(t *testing.T) {
	// При SERIALIZABLE хотя бы одна транзакция должна откатиться
	// Результат: всегда >= 1 активного врача
	pool := testPool(t)
	ctx := context.Background()
	alice, bob := setupDoctors(t, pool)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, doc := range []string{alice, bob} {
		wg.Add(1)
		go func(idx int, d string) {
			defer wg.Done()
			for attempt := range 5 {
				errs[idx] = TryGoOffCall(ctx, pool, d, pgx.Serializable)
				if !isRetryable(errs[idx]) {
					break
				}
				_ = attempt
			}
		}(i, doc)
	}
	wg.Wait()

	remaining := activeCount(t, pool, alice, bob)
	if remaining < 1 {
		t.Errorf("SERIALIZABLE: 0 активных врачей - Write Skew произошёл!")
	}
	t.Logf("Активных врачей: %d, ошибки: %v, %v", remaining, errs[0], errs[1])
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	return IsRetryable(err)
}
