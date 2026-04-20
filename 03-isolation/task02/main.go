// ============================================================
// Задача: Phantom Read - фантомное чтение и Serializable  🟡 Middle
// ============================================================
//
// Phantom Read - аномалия при которой одна транзакция дважды выполняет
// ОДИНАКОВЫЙ запрос с условием (WHERE / COUNT) и получает РАЗНЫЕ наборы строк.
//
// Почему «фантомное»: появляются строки которых раньше не было («фантомы»).
//
// Воспроизведение:
//
//   Транзакция A (REPEATABLE READ):
//     t=1: SELECT COUNT(*) FROM inventory WHERE quantity > 0  → 3
//     t=3: SELECT COUNT(*) FROM inventory WHERE quantity > 0  → 4  ← появилась новая строка!
//
//   Транзакция B:
//     t=2: INSERT INTO inventory (product, quantity) VALUES ('New', 10); COMMIT;
//
//   ВАЖНО: в PostgreSQL REPEATABLE READ уже защищает от фантомных чтений
//   благодаря MVCC! Это отличие от стандарта SQL.
//   Но SERIALIZABLE даёт дополнительные гарантии - защиту от write skew.
//
// Реализуй:
//
//   func CountActive(ctx, tx pgx.Tx) (int, error)
//     - SELECT COUNT(*) FROM inventory WHERE quantity > 0
//
//   func InsertProduct(ctx, pool, product string, quantity int) error
//     - INSERT INTO inventory (product, quantity)
//
//   func ReadTwiceWithInsertBetween(ctx, pool, isoLevel pgx.TxIsoLevel, insertBetween func()) (first, second int, err error)
//     - BEGIN с указанным уровнем изоляции
//     - CountActive (first)
//     - insertBetween()
//     - CountActive (second)
//     - ROLLBACK
//
// Запуск:
//   go mod tidy && go test -v ./...

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func dsn() string {
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		return v
	}
	return "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
}

// TODO: реализуй CountActive - COUNT(*) WHERE quantity > 0
func CountActive(ctx context.Context, tx pgx.Tx) (int, error) {
	// var count int
	// err := tx.QueryRow(ctx, "SELECT COUNT(*) FROM inventory WHERE quantity > 0").Scan(&count)
	// return count, err
	return 0, nil
}

// TODO: реализуй InsertProduct
func InsertProduct(ctx context.Context, pool *pgxpool.Pool, product string, quantity int) error {
	// _, err := pool.Exec(ctx, "INSERT INTO inventory (product, quantity) VALUES ($1, $2)", product, quantity)
	// return err
	return nil
}

// TODO: реализуй ReadTwiceWithInsertBetween
func ReadTwiceWithInsertBetween(ctx context.Context, pool *pgxpool.Pool, isoLevel pgx.TxIsoLevel, insertBetween func()) (first, second int, err error) {
	// tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: isoLevel})
	// if err != nil { return 0, 0, err }
	// defer tx.Rollback(ctx)
	//
	// first, err = CountActive(ctx, tx)
	// if err != nil { return 0, 0, err }
	//
	// insertBetween()
	//
	// second, err = CountActive(ctx, tx)
	// if err != nil { return 0, 0, err }
	//
	// return first, second, nil
	return 0, 0, nil
}

func main() {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	// Подготовка: 3 товара
	pool.Exec(ctx, "DELETE FROM inventory WHERE product LIKE 'phantom-%'")
	for i := range 3 {
		InsertProduct(ctx, pool, fmt.Sprintf("phantom-item-%d", i), 10)
	}
	defer pool.Exec(ctx, "DELETE FROM inventory WHERE product LIKE 'phantom-%'")

	insertNew := func() {
		InsertProduct(ctx, pool, "phantom-new-item", 5)
	}
	defer pool.Exec(ctx, "DELETE FROM inventory WHERE product = 'phantom-new-item'")

	// REPEATABLE READ - в PostgreSQL тоже не видит фантомы (MVCC)
	fmt.Println("=== REPEATABLE READ ===")
	f1, s1, _ := ReadTwiceWithInsertBetween(ctx, pool, pgx.RepeatableRead, insertNew)
	fmt.Printf("Первый COUNT: %d, Второй COUNT: %d\n", f1, s1)
	if f1 == s1 {
		fmt.Println("PostgreSQL REPEATABLE READ: фантомных чтений нет (MVCC)")
	}

	// Убираем новый товар для следующего теста
	pool.Exec(ctx, "DELETE FROM inventory WHERE product = 'phantom-new-item'")

	// SERIALIZABLE
	fmt.Println("\n=== SERIALIZABLE ===")
	f2, s2, _ := ReadTwiceWithInsertBetween(ctx, pool, pgx.Serializable, insertNew)
	fmt.Printf("Первый COUNT: %d, Второй COUNT: %d\n", f2, s2)
	if f2 == s2 {
		fmt.Println("SERIALIZABLE: фантомных чтений нет + защита от write skew")
	}
}
