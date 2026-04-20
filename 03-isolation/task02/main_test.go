package main

import (
	"context"
	"fmt"
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

func TestCountActive(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	prefix := fmt.Sprintf("count-test-%d", time.Now().UnixNano())
	pool.Exec(ctx, "INSERT INTO inventory (product, quantity) VALUES ($1, 5), ($2, 0)", prefix+"-a", prefix+"-b")
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM inventory WHERE product LIKE $1", prefix+"%") })

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	defer tx.Rollback(ctx)

	count, err := CountActive(ctx, tx)
	if err != nil {
		t.Fatalf("CountActive: %v", err)
	}
	if count < 1 {
		t.Errorf("CountActive = %d, хотим >= 1", count)
	}
}

func TestRepeatableReadNoPhantom(t *testing.T) {
	// PostgreSQL REPEATABLE READ защищает от фантомных чтений через MVCC
	pool := testPool(t)
	ctx := context.Background()

	prefix := fmt.Sprintf("phantom-rr-%d", time.Now().UnixNano())
	pool.Exec(ctx, "INSERT INTO inventory (product, quantity) VALUES ($1, 10)", prefix+"-existing")
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM inventory WHERE product LIKE $1", prefix+"%") })

	newProduct := prefix + "-new"
	first, second, err := ReadTwiceWithInsertBetween(ctx, pool, pgx.RepeatableRead, func() {
		InsertProduct(ctx, pool, newProduct, 5)
	})
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM inventory WHERE product = $1", newProduct) })

	if err != nil {
		t.Fatalf("ReadTwiceWithInsertBetween: %v", err)
	}

	// В PostgreSQL REPEATABLE READ = snapshot → фантомов нет
	if first != second {
		t.Errorf("REPEATABLE READ: first=%d, second=%d - фантомное чтение произошло (неожиданно в PG)", first, second)
	}
}

func TestSerializableNoPhantom(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	prefix := fmt.Sprintf("phantom-ser-%d", time.Now().UnixNano())
	pool.Exec(ctx, "INSERT INTO inventory (product, quantity) VALUES ($1, 10)", prefix+"-existing")
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM inventory WHERE product LIKE $1", prefix+"%") })

	newProduct := prefix + "-new"
	first, second, err := ReadTwiceWithInsertBetween(ctx, pool, pgx.Serializable, func() {
		InsertProduct(ctx, pool, newProduct, 5)
	})
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM inventory WHERE product = $1", newProduct) })

	if err != nil {
		t.Fatalf("ReadTwiceWithInsertBetween: %v", err)
	}

	if first != second {
		t.Errorf("SERIALIZABLE: first=%d, second=%d - фантомное чтение (неожиданно)", first, second)
	}
}
