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

func uniqueEmail() string {
	return fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())
}

func TestInsertBatchAllValid(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	items := []Item{
		{Name: "A", Email: uniqueEmail()},
		{Name: "B", Email: uniqueEmail()},
		{Name: "C", Email: uniqueEmail()},
	}

	inserted, skipped, err := InsertBatch(ctx, pool, items)
	if err != nil {
		t.Fatalf("InsertBatch: %v", err)
	}
	t.Cleanup(func() {
		for _, it := range items {
			pool.Exec(ctx, "DELETE FROM users WHERE email = $1", it.Email)
		}
	})

	if inserted != 3 {
		t.Errorf("inserted = %d, хотим 3", inserted)
	}
	if skipped != 0 {
		t.Errorf("skipped = %d, хотим 0", skipped)
	}
}

func TestInsertBatchWithDuplicate(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	dupEmail := uniqueEmail()
	// Вставляем запись заранее чтобы создать конфликт
	pool.Exec(ctx, "INSERT INTO users (name, email) VALUES ('Dup', $1)", dupEmail)
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM users WHERE email = $1", dupEmail)
	})

	goodEmail1 := uniqueEmail()
	goodEmail2 := uniqueEmail()

	items := []Item{
		{Name: "Good1", Email: goodEmail1},
		{Name: "Conflict", Email: dupEmail},  // дубликат
		{Name: "Good2", Email: goodEmail2},
	}

	inserted, skipped, err := InsertBatch(ctx, pool, items)
	if err != nil {
		t.Fatalf("InsertBatch: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM users WHERE email IN ($1, $2)", goodEmail1, goodEmail2)
	})

	if inserted != 2 {
		t.Errorf("inserted = %d, хотим 2 (пропустили дубликат)", inserted)
	}
	if skipped != 1 {
		t.Errorf("skipped = %d, хотим 1", skipped)
	}

	// Проверяем что Good1 и Good2 реально в БД
	var count int
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE email IN ($1, $2)", goodEmail1, goodEmail2).Scan(&count)
	if count != 2 {
		t.Errorf("в БД найдено %d записей, хотим 2 (savepoint сохранил корректные)", count)
	}
}

func TestInsertBatchAllDuplicates(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	email := uniqueEmail()
	pool.Exec(ctx, "INSERT INTO users (name, email) VALUES ('Dup', $1)", email)
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM users WHERE email = $1", email) })

	items := []Item{
		{Name: "X", Email: email},
		{Name: "Y", Email: email},
	}

	inserted, skipped, err := InsertBatch(ctx, pool, items)
	if err != nil {
		t.Fatalf("InsertBatch: %v", err)
	}
	if inserted != 0 {
		t.Errorf("inserted = %d, хотим 0", inserted)
	}
	if skipped != 2 {
		t.Errorf("skipped = %d, хотим 2", skipped)
	}
}
