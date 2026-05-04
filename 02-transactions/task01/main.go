// ============================================================
// Задача: Транзакции - перевод денег (BEGIN/COMMIT/ROLLBACK)  🟡 Middle
// ============================================================
//
// Транзакция = группа операций, выполняемых атомарно:
//   Либо ВСЕ операции выполнились → COMMIT
//   Либо НИ ОДНА не применилась   → ROLLBACK
//
// ACID:
//   A - Atomicity    (Атомарность): всё или ничего
//   C - Consistency  (Согласованность): БД остаётся в корректном состоянии
//   I - Isolation    (Изолированность): транзакции не мешают друг другу
//   D - Durability   (Долговечность): COMMIT = данные сохранены навсегда
//
// Пример - перевод денег:
//   BEGIN;
//   UPDATE accounts SET balance = balance - 100 WHERE id = 1;  -- снять с Alice
//   UPDATE accounts SET balance = balance + 100 WHERE id = 2;  -- добавить Bob'у
//   COMMIT;
//
//   Если после первого UPDATE сервис упал → ROLLBACK автоматически.
//   Деньги НЕ потеряются (нет ни списания, ни начисления).
//
// Идиоматичный паттерн в Go:
//   tx, err := pool.Begin(ctx)
//   defer tx.Rollback(ctx)  // безопасно вызывать после Commit - ничего не делает
//   ...операции...
//   tx.Commit(ctx)
//
// Реализуй:
//
//   func Transfer(ctx, pool, fromID, toID int64, amount float64) error
//     - BEGIN
//     - SELECT balance FROM accounts WHERE id = $1 FOR UPDATE
//       (FOR UPDATE: блокируем строку, чтобы параллельный перевод не прочитал старое значение)
//     - проверить что fromID имеет достаточно средств
//     - UPDATE accounts SET balance = balance - $1 WHERE id = $2
//     - UPDATE accounts SET balance = balance + $1 WHERE id = $3
//     - COMMIT
//
//   func GetBalance(ctx, pool, id int64) (float64, error)
//     - SELECT balance FROM accounts WHERE id = $1
//
// Запуск:
//   go mod tidy && go test -v ./...

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInsufficientFunds = errors.New("insufficient funds")

func dsn() string {
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		return v
	}
	return "postgres://postgres:postgres@localhost:5433/postgres?sslmode=disable"
}

// TODO: реализуй Transfer - атомарный перевод средств
func Transfer(ctx context.Context, pool *pgxpool.Pool, fromID, toID int64, amount float64) error {
	// Алгоритм:
	// Начинаем транзакцию
	// Begin(ctx) получает соединение из пула и начинает транзакцию
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	// Идиома Go: отложенный откат
	// Если будет вызван tx.Commit() — Rollback проигнорируется
	// Если выйдем по ошибке — Rollback откатит изменения
	defer tx.Rollback(ctx)

	// Блокируем строки в ОДНОМ порядке (min(fromID, toID) first) - защита от deadlock!
	var balance float64
	err = tx.QueryRow(ctx,
		"SELECT balance FROM accounts WHERE id = $1 FOR UPDATE", fromID,
	).Scan(&balance)
	if err != nil {
		return err // Транзакция откатится через defer Rollback
	}
	// Проверяем, достаточно ли денег на счёте
	if balance < amount {
		return ErrInsufficientFunds // Откатываем транзакцию
	}
	// UPDATE с вычислением: новый_баланс = старый_баланс - amount
	// Exec — выполнение запроса без возврата строк
	_, err = tx.Exec(ctx,
		"UPDATE accounts SET balance = balance - $1 WHERE id = $2", amount, fromID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx,
		"UPDATE accounts SET balance = balance + $1 WHERE id = $2", amount, toID)
	if err != nil {
		return err
	}
	// Commit — применяем все изменения
	// Если ошибка — defer Rollback откатит всё
	return tx.Commit(ctx)
}

// TODO: реализуй GetBalance
func GetBalance(ctx context.Context, pool *pgxpool.Pool, id int64) (float64, error) {
	var balance float64
	// QueryRow — запрос, возвращающий одну строку
	err := pool.QueryRow(ctx, "SELECT balance FROM accounts WHERE id = $1", id).Scan(&balance)
	// Если пользователь не найден — вернёт ошибку sql.ErrNoRows
	return balance, err
}

func setupAccounts(ctx context.Context, pool *pgxpool.Pool) (int64, int64, error) {
	var aliceID, bobID int64
	// INSERT ... RETURNING id — вставляет и сразу возвращает id
	err := pool.QueryRow(ctx,
		"INSERT INTO accounts (owner, balance) VALUES ('Alice', 1000) RETURNING id",
	).Scan(&aliceID)
	if err != nil {
		return 0, 0, err
	}
	err = pool.QueryRow(ctx,
		"INSERT INTO accounts (owner, balance) VALUES ('Bob', 500) RETURNING id",
	).Scan(&bobID)
	return aliceID, bobID, err
}

func main() {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		log.Fatalf("connect: %v\nЗапусти: docker compose up -d", err)
	}
	defer pool.Close()

	aliceID, bobID, err := setupAccounts(ctx, pool)
	if err != nil {
		log.Fatalf("setup: %v", err)
	}
	defer pool.Exec(ctx, "DELETE FROM accounts WHERE id IN ($1, $2)", aliceID, bobID)

	// Успешный перевод
	fmt.Println("=== Перевод 200 от Alice к Bob ===")
	aliceBefore, _ := GetBalance(ctx, pool, aliceID)
	bobBefore, _ := GetBalance(ctx, pool, bobID)
	fmt.Printf("До:   Alice=%.2f Bob=%.2f\n", aliceBefore, bobBefore)

	if err := Transfer(ctx, pool, aliceID, bobID, 200); err != nil {
		log.Fatalf("Transfer: %v", err)
	}

	aliceAfter, _ := GetBalance(ctx, pool, aliceID)
	bobAfter, _ := GetBalance(ctx, pool, bobID)
	fmt.Printf("После: Alice=%.2f Bob=%.2f\n", aliceAfter, bobAfter)

	// Недостаточно средств - должен откатиться
	fmt.Println("\n=== Перевод 99999 (недостаточно средств) ===")
	err = Transfer(ctx, pool, aliceID, bobID, 99999)
	if errors.Is(err, ErrInsufficientFunds) {
		fmt.Println("Ошибка: недостаточно средств (ожидаемо)")
	} else {
		fmt.Printf("Неожиданная ошибка: %v\n", err)
	}

	aliceFinal, _ := GetBalance(ctx, pool, aliceID)
	bobFinal, _ := GetBalance(ctx, pool, bobID)
	fmt.Printf("Баланс не изменился: Alice=%.2f Bob=%.2f\n", aliceFinal, bobFinal)
}
