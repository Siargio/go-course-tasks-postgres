package main

import (
	"context"
	"testing"
	"time"
)

func testPool(t *testing.T) interface{ Close() } {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pool, err := Connect(ctx, DSN())
	if err != nil {
		t.Skipf("PostgreSQL недоступен - запусти: docker compose up -d\nОшибка: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestConnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pool, err := Connect(ctx, DSN())
	if err != nil {
		t.Skipf("PostgreSQL недоступен: %v", err)
	}
	defer pool.Close()

	if pool == nil {
		t.Fatal("Connect вернул nil - реализуй функцию")
	}
}

func TestConnectPingFails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	badDSN := "postgres://postgres:wrong@localhost:9999/noexist?sslmode=disable"
	pool, err := Connect(ctx, badDSN)
	if err == nil {
		if pool != nil {
			pool.Close()
		}
		t.Error("Connect с неверным DSN должен вернуть ошибку")
	}
}

func TestConnectWithConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pool, err := ConnectWithConfig(ctx, DSN(), 5)
	if err != nil {
		t.Skipf("PostgreSQL недоступен: %v", err)
	}
	defer pool.Close()

	if pool == nil {
		t.Fatal("ConnectWithConfig вернул nil - реализуй функцию")
	}

	stats := pool.Stat()
	if stats.MaxConns() != 5 {
		t.Errorf("MaxConns = %d, хотим 5", stats.MaxConns())
	}
}

func TestDSNReturnsNonEmpty(t *testing.T) {
	dsn := DSN()
	if dsn == "" {
		t.Error("DSN() не должен возвращать пустую строку")
	}
}
