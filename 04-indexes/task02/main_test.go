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

func TestInsertOrderWithMeta(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	id, err := InsertOrderWithMeta(ctx, pool, 1, `{"tag":"test"}`)
	if err != nil {
		t.Fatalf("InsertOrderWithMeta: %v", err)
	}
	if id <= 0 {
		t.Errorf("id = %d, хотим > 0", id)
	}
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM orders WHERE id = $1", id) })
}

func TestSearchByMetadata(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tag := fmt.Sprintf("tag-%d", time.Now().UnixNano())
	meta1 := fmt.Sprintf(`{"tag":"%s","region":"EU"}`, tag)
	meta2 := fmt.Sprintf(`{"tag":"%s","region":"US"}`, tag)
	meta3 := `{"tag":"other"}`

	id1, _ := InsertOrderWithMeta(ctx, pool, 1, meta1)
	id2, _ := InsertOrderWithMeta(ctx, pool, 2, meta2)
	id3, _ := InsertOrderWithMeta(ctx, pool, 3, meta3)
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM orders WHERE id IN ($1, $2, $3)", id1, id2, id3)
	})

	filter := fmt.Sprintf(`{"tag":"%s"}`, tag)
	ids, err := SearchByMetadata(ctx, pool, filter)
	if err != nil {
		t.Fatalf("SearchByMetadata: %v", err)
	}

	if len(ids) != 2 {
		t.Errorf("SearchByMetadata вернул %d результатов, хотим 2", len(ids))
	}
}

func TestCreateGINIndex(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	t.Cleanup(func() {
		pool.Exec(ctx, "DROP INDEX IF EXISTS idx_orders_metadata")
	})

	if err := CreateGINIndex(ctx, pool); err != nil {
		t.Fatalf("CreateGINIndex: %v", err)
	}

	var count int
	pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM pg_indexes
		WHERE tablename = 'orders' AND indexname = 'idx_orders_metadata'
	`).Scan(&count)

	if count != 1 {
		t.Error("GIN индекс idx_orders_metadata не создан")
	}
}

func TestGINIndexUsedInPlan(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	// Вставляем данные для получения реального плана
	var ids []int64
	for i := range 50 {
		id, _ := InsertOrderWithMeta(ctx, pool, i+1, `{"tag":"gin-test"}`)
		ids = append(ids, id)
	}
	t.Cleanup(func() {
		for _, id := range ids {
			pool.Exec(ctx, "DELETE FROM orders WHERE id = $1", id)
		}
		pool.Exec(ctx, "DROP INDEX IF EXISTS idx_orders_metadata")
	})

	CreateGINIndex(ctx, pool)
	pool.Exec(ctx, "ANALYZE orders")

	plan, err := ExplainSearchByMeta(ctx, pool, `{"tag":"gin-test"}`)
	if err != nil {
		t.Fatalf("ExplainSearchByMeta: %v", err)
	}
	if plan == "" {
		t.Error("ExplainSearchByMeta вернул пустой план")
	}
	t.Logf("Plan:\n%s", plan)
}
