package main

import (
	"context"
	"fmt"
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

func TestEnqueue(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	id, err := Enqueue(ctx, pool, "test-payload")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if id <= 0 {
		t.Errorf("Enqueue вернул id=%d, хотим > 0", id)
	}
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM jobs WHERE id = $1", id) })
}

func TestDequeueReturnsJob(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	payload := fmt.Sprintf("dequeue-test-%d", time.Now().UnixNano())
	id, err := Enqueue(ctx, pool, payload)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM jobs WHERE id = $1", id) })

	job, err := Dequeue(ctx, pool)
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if job == nil {
		t.Fatal("Dequeue вернул nil - реализуй функцию")
	}
	if job.ID != id {
		t.Errorf("job.ID = %d, хотим %d", job.ID, id)
	}

	// Статус должен стать 'processing'
	var status string
	pool.QueryRow(ctx, "SELECT status FROM jobs WHERE id = $1", id).Scan(&status)
	if status != "processing" {
		t.Errorf("status = %q, хотим 'processing'", status)
	}
}

func TestDequeueReturnsNilWhenEmpty(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	// Очищаем pending задачи
	pool.Exec(ctx, "DELETE FROM jobs WHERE status = 'pending'")

	job, err := Dequeue(ctx, pool)
	if err != nil {
		t.Fatalf("Dequeue (пусто): %v", err)
	}
	if job != nil {
		t.Errorf("Dequeue (пусто) вернул %+v, хотим nil", job)
		pool.Exec(ctx, "DELETE FROM jobs WHERE id = $1", job.ID)
	}
}

func TestCompleteAndFail(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	id1, _ := Enqueue(ctx, pool, "complete-me")
	id2, _ := Enqueue(ctx, pool, "fail-me")
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM jobs WHERE id IN ($1, $2)", id1, id2)
	})

	if err := Complete(ctx, pool, id1); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if err := Fail(ctx, pool, id2); err != nil {
		t.Fatalf("Fail: %v", err)
	}

	var s1, s2 string
	pool.QueryRow(ctx, "SELECT status FROM jobs WHERE id = $1", id1).Scan(&s1)
	pool.QueryRow(ctx, "SELECT status FROM jobs WHERE id = $1", id2).Scan(&s2)

	if s1 != "done" {
		t.Errorf("после Complete статус = %q, хотим 'done'", s1)
	}
	if s2 != "failed" {
		t.Errorf("после Fail статус = %q, хотим 'failed'", s2)
	}
}

func TestSkipLockedNoDuplicateProcessing(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	// Кладём 20 задач
	const total = 20
	var ids []int64
	for i := range total {
		id, err := Enqueue(ctx, pool, fmt.Sprintf("job-%d", i))
		if err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
		ids = append(ids, id)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM jobs WHERE id = ANY($1)", ids)
	})

	// 5 параллельных воркеров
	var (
		wg        sync.WaitGroup
		processed atomic.Int32
	)
	for w := range 5 {
		wg.Add(1)
		go func(wid int) {
			defer wg.Done()
			for {
				job, err := Dequeue(ctx, pool)
				if err != nil || job == nil {
					return
				}
				Complete(ctx, pool, job.ID)
				processed.Add(1)
				_ = wid
			}
		}(w)
	}
	wg.Wait()

	if int(processed.Load()) != total {
		t.Errorf("обработано %d задач, хотим %d (SKIP LOCKED не работает - задачи дублируются или теряются)", processed.Load(), total)
	}
}
