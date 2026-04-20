package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		code string
		want bool
	}{
		{pgSerializationFailure, true},
		{pgDeadlockDetected, true},
		{"23505", false}, // unique_violation
		{"", false},
	}

	for _, tc := range tests {
		var err error
		if tc.code != "" {
			err = &pgconn.PgError{Code: tc.code}
		} else {
			err = errors.New("generic error")
		}
		got := IsRetryable(err)
		if got != tc.want {
			t.Errorf("IsRetryable(code=%q) = %v, хотим %v", tc.code, got, tc.want)
		}
	}
}

func TestRunWithRetrySuccess(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	counterName := fmt.Sprintf("test-retry-%d", time.Now().UnixNano())
	pool.Exec(ctx, "INSERT INTO counters (name, value) VALUES ($1, 0)", counterName)
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM counters WHERE name = $1", counterName) })

	err := RunWithRetry(ctx, pool, 3, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "UPDATE counters SET value = value + 1 WHERE name = $1", counterName)
		return err
	})
	if err != nil {
		t.Fatalf("RunWithRetry: %v", err)
	}

	var val int
	pool.QueryRow(ctx, "SELECT value FROM counters WHERE name = $1", counterName).Scan(&val)
	if val != 1 {
		t.Errorf("value = %d, хотим 1", val)
	}
}

func TestRunWithRetryPermanentError(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	sentinelErr := errors.New("permanent error")
	err := RunWithRetry(ctx, pool, 5, func(tx pgx.Tx) error {
		return sentinelErr
	})
	if !errors.Is(err, sentinelErr) {
		t.Errorf("ожидали sentinelErr, получили: %v", err)
	}
}

func TestRunWithRetryConcurrentUpdates(t *testing.T) {
	// Проверяем что конкурентные SERIALIZABLE транзакции не теряют обновления
	pool := testPool(t)
	ctx := context.Background()

	counterName := fmt.Sprintf("test-concurrent-%d", time.Now().UnixNano())
	pool.Exec(ctx, "INSERT INTO counters (name, value) VALUES ($1, 0)", counterName)
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM counters WHERE name = $1", counterName) })

	n := 5
	var wg sync.WaitGroup
	errs := make([]error, n)

	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = RunWithRetry(ctx, pool, 10, func(tx pgx.Tx) error {
				var val int
				tx.QueryRow(ctx, "SELECT value FROM counters WHERE name = $1", counterName).Scan(&val)
				_, err := tx.Exec(ctx, "UPDATE counters SET value = $1 WHERE name = $2", val+1, counterName)
				return err
			})
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("горутина %d: %v", i, err)
		}
	}

	var finalVal int
	pool.QueryRow(ctx, "SELECT value FROM counters WHERE name = $1", counterName).Scan(&finalVal)
	if finalVal != n {
		t.Errorf("финальное значение = %d, хотим %d (потеряны обновления при конкурентном доступе)", finalVal, n)
	}
}
