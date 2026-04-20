package main

import (
	"context"
	"errors"
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
		t.Skipf("PostgreSQL недоступен - запусти: docker compose up -d\nОшибка: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("ping failed: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func createAccounts(t *testing.T, pool *pgxpool.Pool, aliceBalance, bobBalance float64) (int64, int64) {
	t.Helper()
	ctx := context.Background()
	var aliceID, bobID int64
	pool.QueryRow(ctx, "INSERT INTO accounts (owner, balance) VALUES ('Alice', $1) RETURNING id", aliceBalance).Scan(&aliceID)
	pool.QueryRow(ctx, "INSERT INTO accounts (owner, balance) VALUES ('Bob', $1) RETURNING id", bobBalance).Scan(&bobID)
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM accounts WHERE id IN ($1, $2)", aliceID, bobID)
	})
	return aliceID, bobID
}

func TestTransferSuccess(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	aliceID, bobID := createAccounts(t, pool, 1000, 500)

	if err := Transfer(ctx, pool, aliceID, bobID, 300); err != nil {
		t.Fatalf("Transfer: %v", err)
	}

	aliceBal, _ := GetBalance(ctx, pool, aliceID)
	bobBal, _ := GetBalance(ctx, pool, bobID)

	if aliceBal != 700 {
		t.Errorf("Alice balance = %.2f, хотим 700", aliceBal)
	}
	if bobBal != 800 {
		t.Errorf("Bob balance = %.2f, хотим 800", bobBal)
	}
}

func TestTransferInsufficientFunds(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	aliceID, bobID := createAccounts(t, pool, 100, 0)

	err := Transfer(ctx, pool, aliceID, bobID, 500)
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Errorf("ожидали ErrInsufficientFunds, получили: %v", err)
	}

	// Балансы не изменились
	aliceBal, _ := GetBalance(ctx, pool, aliceID)
	if aliceBal != 100 {
		t.Errorf("после неудачного перевода Alice balance = %.2f, хотим 100", aliceBal)
	}
}

func TestTransferAtomicity(t *testing.T) {
	// Если сумма снята - должна быть зачислена (атомарность)
	// Проверяем через сумму балансов: до и после должна совпадать
	pool := testPool(t)
	ctx := context.Background()

	aliceID, bobID := createAccounts(t, pool, 1000, 1000)

	total := func() float64 {
		a, _ := GetBalance(ctx, pool, aliceID)
		b, _ := GetBalance(ctx, pool, bobID)
		return a + b
	}

	before := total()
	Transfer(ctx, pool, aliceID, bobID, 300)
	after := total()

	if before != after {
		t.Errorf("сумма балансов изменилась: до=%.2f, после=%.2f (нарушение атомарности)", before, after)
	}
}

func TestGetBalance(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	aliceID, _ := createAccounts(t, pool, 250.50, 0)

	bal, err := GetBalance(ctx, pool, aliceID)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if bal != 250.50 {
		t.Errorf("balance = %.2f, хотим 250.50", bal)
	}
}
