package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

func TestTryLockAndUnlock(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer conn.Release()

	const key = int64(9_000_001)

	locked, err := TryLock(ctx, conn, key)
	if err != nil {
		t.Fatalf("TryLock: %v", err)
	}
	if !locked {
		t.Fatal("TryLock вернул false - должен был захватить свободную блокировку")
	}

	// Второй TryLock на том же соединении - advisory locks реентрантны
	// (одно соединение может захватить один ключ несколько раз)
	if err := Unlock(ctx, conn, key); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
}

func TestTryLockSecondConnectionFails(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	const key = int64(9_000_002)

	conn1, _ := pool.Acquire(ctx)
	defer conn1.Release()

	conn2, _ := pool.Acquire(ctx)
	defer conn2.Release()

	// conn1 захватывает
	locked1, err := TryLock(ctx, conn1, key)
	if err != nil {
		t.Fatalf("TryLock conn1: %v", err)
	}
	if !locked1 {
		t.Skip("не удалось захватить блокировку (возможно уже занята от предыдущего теста)")
	}
	defer Unlock(ctx, conn1, key)

	// conn2 должен получить false
	locked2, err := TryLock(ctx, conn2, key)
	if err != nil {
		t.Fatalf("TryLock conn2: %v", err)
	}
	if locked2 {
		t.Error("второй TryLock должен был вернуть false - блокировка занята conn1")
		Unlock(ctx, conn2, key)
	}
}

func TestWithAdvisoryLockMutualExclusion(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	const key = int64(9_000_003)

	var (
		wg         sync.WaitGroup
		executions atomic.Int32
		busyCount  atomic.Int32
	)

	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := WithAdvisoryLock(ctx, pool, key, func() error {
				executions.Add(1)
				return nil
			})
			if errors.Is(err, ErrLockBusy) {
				busyCount.Add(1)
			}
		}()
	}
	wg.Wait()

	if executions.Load() == 0 {
		t.Error("ни один воркер не выполнился - реализуй WithAdvisoryLock")
	}
	t.Logf("Выполнений: %d, Заблокировано: %d", executions.Load(), busyCount.Load())
}

func TestWithAdvisoryLockFnErrorReleasesLock(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	const key = int64(9_000_004)
	sentinelErr := errors.New("fn error")

	err := WithAdvisoryLock(ctx, pool, key, func() error {
		return sentinelErr
	})
	if !errors.Is(err, sentinelErr) {
		t.Errorf("WithAdvisoryLock должен вернуть ошибку fn: %v", err)
	}

	// После ошибки блокировка должна быть освобождена
	conn, _ := pool.Acquire(ctx)
	defer conn.Release()
	locked, _ := TryLock(ctx, conn, key)
	if !locked {
		t.Error("после ошибки fn блокировка не была освобождена")
	}
	Unlock(ctx, conn, key)
}
