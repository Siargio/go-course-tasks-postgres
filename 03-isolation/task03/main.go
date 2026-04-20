// ============================================================
// Задача: Write Skew - аномалия при которой нужен Serializable  🔴 Senior
// ============================================================
//
// Write Skew - самая тонкая аномалия изоляции.
// Не защищена ни READ COMMITTED, ни REPEATABLE READ.
// Только SERIALIZABLE её устраняет.
//
// Классический пример - дежурные врачи:
//
//   Правило: в любой момент дежурит хотя бы 1 врач (active=true).
//
//   Начальное состояние: Dr.Alice (active), Dr.Bob (active) - 2 дежурных.
//
//   Транзакция A (снять Alice):
//     SELECT COUNT(*) FROM on_call WHERE active=true  → 2  (OK, > 1)
//     UPDATE on_call SET active=false WHERE doctor='Alice'
//
//   Транзакция B (снять Bob) - параллельно с A:
//     SELECT COUNT(*) FROM on_call WHERE active=true  → 2  (тоже OK!)
//     UPDATE on_call SET active=false WHERE doctor='Bob'
//
//   Результат: 0 дежурных! Правило нарушено.
//
//   Почему это Write Skew а не Lost Update?
//     А и B читают ОДНИ строки, но пишут РАЗНЫЕ.
//     Нет "потери обновления" - у каждой транзакции своя строка.
//     Но вместе они нарушают инвариант системы.
//
//   Почему REPEATABLE READ не помогает?
//     REPEATABLE READ видит снимок на момент BEGIN.
//     A видит 2 дежурных → решает что можно уйти.
//     B видит 2 дежурных → тоже решает что можно уйти.
//     Оба снимка "правдивы" на момент чтения, но результат неверен.
//
//   SERIALIZABLE обнаруживает эту зависимость (SSI - Serializable Snapshot Isolation)
//   и откатывает одну из транзакций с ошибкой 40001.
//
// Реализуй:
//
//   func TryGoOffCall(ctx, pool, doctor string, isoLevel pgx.TxIsoLevel) error
//     - BEGIN с isoLevel
//     - SELECT COUNT(*) FROM on_call WHERE active=true
//     - если count > 1: UPDATE on_call SET active=false WHERE doctor=$1
//     - иначе: return ErrLastDoctor
//     - COMMIT
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
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrLastDoctor = errors.New("cannot go off-call: last active doctor")

func dsn() string {
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		return v
	}
	return "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
}

// TODO: реализуй TryGoOffCall
func TryGoOffCall(ctx context.Context, pool *pgxpool.Pool, doctor string, isoLevel pgx.TxIsoLevel) error {
	// tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: isoLevel})
	// if err != nil { return err }
	// defer tx.Rollback(ctx)
	//
	// var count int
	// tx.QueryRow(ctx, "SELECT COUNT(*) FROM on_call WHERE active = true").Scan(&count)
	//
	// if count <= 1 {
	//     return ErrLastDoctor
	// }
	//
	// tx.Exec(ctx, "UPDATE on_call SET active = false WHERE doctor = $1", doctor)
	// return tx.Commit(ctx)
	return nil
}

func main() {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	setup := func() (int64, int64) {
		var aliceID, bobID int64
		pool.Exec(ctx, "DELETE FROM on_call WHERE doctor IN ('Alice', 'Bob')")
		pool.QueryRow(ctx, "INSERT INTO on_call (doctor, active) VALUES ('Alice', true) RETURNING id").Scan(&aliceID)
		pool.QueryRow(ctx, "INSERT INTO on_call (doctor, active) VALUES ('Bob', true) RETURNING id").Scan(&bobID)
		return aliceID, bobID
	}

	activeCount := func() int {
		var n int
		pool.QueryRow(ctx, "SELECT COUNT(*) FROM on_call WHERE active = true AND doctor IN ('Alice', 'Bob')").Scan(&n)
		return n
	}

	// === REPEATABLE READ - Write Skew возможен ===
	fmt.Println("=== REPEATABLE READ (write skew возможен) ===")
	aliceID, bobID := setup()
	defer pool.Exec(ctx, "DELETE FROM on_call WHERE id IN ($1, $2)", aliceID, bobID)

	var wg sync.WaitGroup
	for _, doc := range []string{"Alice", "Bob"} {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			err := TryGoOffCall(ctx, pool, d, pgx.RepeatableRead)
			fmt.Printf("  %s: %v\n", d, err)
		}(doc)
	}
	wg.Wait()
	fmt.Printf("Дежурных осталось: %d (должно быть >= 1, но может быть 0 - Write Skew!)\n", activeCount())

	// === SERIALIZABLE - Write Skew предотвращён ===
	fmt.Println("\n=== SERIALIZABLE (write skew предотвращён) ===")
	pool.Exec(ctx, "UPDATE on_call SET active = true WHERE doctor IN ('Alice', 'Bob')")

	var mu sync.Mutex
	results := map[string]error{}
	for _, doc := range []string{"Alice", "Bob"} {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			err := TryGoOffCall(ctx, pool, d, pgx.Serializable)
			mu.Lock()
			results[d] = err
			mu.Unlock()
		}(doc)
	}
	wg.Wait()

	for doc, err := range results {
		fmt.Printf("  %s: %v\n", doc, err)
	}
	fmt.Printf("Дежурных осталось: %d (должно быть 1)\n", activeCount())
}
