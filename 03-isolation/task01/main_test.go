package main

import (
	"context"
	"fmt"
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

func setupCounter(t *testing.T, pool *pgxpool.Pool, initialValue int) string {
	t.Helper()
	name := fmt.Sprintf("iso-test-%d", time.Now().UnixNano())
	ctx := context.Background()
	pool.Exec(ctx, "INSERT INTO counters (name, value) VALUES ($1, $2)", name, initialValue)
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM counters WHERE name = $1", name) })
	return name
}

func TestReadCommittedSeesExternalUpdate(t *testing.T) {
	// READ COMMITTED должен увидеть изменение от другой транзакции
	pool := testPool(t)
	ctx := context.Background()

	name := setupCounter(t, pool, 100)

	updated := false
	first, second, err := ReadTwiceReadCommitted(ctx, pool, name, func() {
		pool.Exec(ctx, "UPDATE counters SET value = 200 WHERE name = $1", name)
		updated = true
	})

	if err != nil {
		t.Fatalf("ReadTwiceReadCommitted: %v", err)
	}
	if !updated {
		t.Skip("updateBetween не был вызван")
	}

	if first != 100 {
		t.Errorf("первый SELECT = %d, хотим 100", first)
	}
	// При READ COMMITTED второй SELECT должен увидеть 200
	if second != 200 {
		t.Errorf("READ COMMITTED второй SELECT = %d, хотим 200 (non-repeatable read ожидаем)", second)
	}
}

func TestRepeatableReadIgnoresExternalUpdate(t *testing.T) {
	// REPEATABLE READ НЕ ДОЛЖЕН видеть изменения, закоммиченные после начала транзакции
	pool := testPool(t)
	ctx := context.Background()

	name := setupCounter(t, pool, 100)

	first, second, err := ReadTwiceRepeatableRead(ctx, pool, name, func() {
		pool.Exec(ctx, "UPDATE counters SET value = 200 WHERE name = $1", name)
	})

	if err != nil {
		t.Fatalf("ReadTwiceRepeatableRead: %v", err)
	}

	if first != 100 {
		t.Errorf("первый SELECT = %d, хотим 100", first)
	}
	// При REPEATABLE READ второй SELECT должен вернуть то же значение
	if first != second {
		t.Errorf("REPEATABLE READ: first=%d, second=%d - должны быть равны (non-repeatable read произошёл)", first, second)
	}
}
