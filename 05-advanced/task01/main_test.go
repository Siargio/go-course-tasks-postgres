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

func TestNotifyAndListen(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	channel := fmt.Sprintf("test-chan-%d", time.Now().UnixNano())
	received := make(chan string, 5)

	listenCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		Listen(listenCtx, dsn(), channel, func(payload string) {
			received <- payload
		})
	}()

	time.Sleep(150 * time.Millisecond)

	payloads := []string{"hello", "world", "postgres"}
	for _, p := range payloads {
		if err := Notify(ctx, pool, channel, p); err != nil {
			t.Fatalf("Notify(%q): %v", p, err)
		}
	}

	for i, want := range payloads {
		select {
		case got := <-received:
			if got != want {
				t.Errorf("[%d] received %q, хотим %q", i, got, want)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("таймаут ожидания уведомления %q - реализуй Listen", want)
		}
	}
}

func TestListenCancelStops(t *testing.T) {
	testPool(t) // проверяем доступность

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- Listen(ctx, dsn(), "test-cancel-chan", func(string) {})
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Listen после отмены вернул ошибку: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Listen не завершился после отмены контекста")
	}
}

func TestTriggerNotifiesOnJobInsert(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	received := make(chan string, 1)
	listenCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		Listen(listenCtx, dsn(), "new_job", func(payload string) {
			received <- payload
		})
	}()

	time.Sleep(150 * time.Millisecond)

	var jobID int64
	pool.QueryRow(ctx, "INSERT INTO jobs (payload) VALUES ('test') RETURNING id").Scan(&jobID)
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM jobs WHERE id = $1", jobID) })

	select {
	case payload := <-received:
		if payload != fmt.Sprintf("%d", jobID) {
			t.Errorf("trigger NOTIFY payload = %q, хотим %q", payload, fmt.Sprintf("%d", jobID))
		}
	case <-time.After(3 * time.Second):
		t.Error("тригер не отправил NOTIFY - проверь init.sql")
	}
}
