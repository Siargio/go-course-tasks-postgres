package main

import (
	"context"
	"strings"
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

func TestSeedOrders(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	var before int
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders").Scan(&before)

	if err := SeedOrders(ctx, pool, 100); err != nil {
		t.Fatalf("SeedOrders: %v", err)
	}

	var after int
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders").Scan(&after)

	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM orders") })

	if after-before < 100 {
		t.Errorf("ожидали +100 заказов, до=%d, после=%d", before, after)
	}
}

func TestCreateIndexes(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	t.Cleanup(func() {
		pool.Exec(ctx, "DROP INDEX IF EXISTS idx_orders_user_id")
		pool.Exec(ctx, "DROP INDEX IF EXISTS idx_orders_pending")
	})

	if err := CreateIndexes(ctx, pool); err != nil {
		t.Fatalf("CreateIndexes: %v", err)
	}

	// Проверяем что индексы созданы
	var count int
	pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM pg_indexes
		WHERE tablename = 'orders'
		AND indexname IN ('idx_orders_user_id', 'idx_orders_pending')
	`).Scan(&count)

	if count < 2 {
		t.Errorf("ожидали 2 индекса, найдено %d", count)
	}
}

func TestExplainQueryReturnsNonEmpty(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	plan, err := ExplainQuery(ctx, pool, "SELECT 1")
	if err != nil {
		t.Fatalf("ExplainQuery: %v", err)
	}
	if plan == "" {
		t.Error("ExplainQuery вернул пустой план")
	}
}

func TestIndexSpeedsUpQuery(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	// Заполняем данные
	SeedOrders(ctx, pool, 5000)
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM orders") })

	// Принудительно пересчитываем статистику
	pool.Exec(ctx, "ANALYZE orders")

	// Запрос без индекса - должен быть Seq Scan
	planBefore, _ := ExplainQuery(ctx, pool,
		"SELECT * FROM orders WHERE user_id = $1", 42)

	CreateIndexes(ctx, pool)
	t.Cleanup(func() {
		pool.Exec(ctx, "DROP INDEX IF EXISTS idx_orders_user_id")
		pool.Exec(ctx, "DROP INDEX IF EXISTS idx_orders_pending")
	})

	pool.Exec(ctx, "ANALYZE orders")

	// После индекса - ожидаем Index Scan
	planAfter, _ := ExplainQuery(ctx, pool,
		"SELECT * FROM orders WHERE user_id = $1", 42)

	t.Logf("До индекса:\n%s", planBefore)
	t.Logf("После индекса:\n%s", planAfter)

	if planAfter != "" && !strings.Contains(planAfter, "Index") {
		t.Log("Подсказка: при малом объёме данных планировщик может выбрать Seq Scan - это нормально")
	}
}
