// ============================================================
// Задача: Savepoints - частичный откат внутри транзакции  🟡 Middle
// ============================================================
//
// Проблема: что если нужно откатить только часть операций внутри транзакции?
//
// Обычный ROLLBACK откатывает ВСЁ. Savepoint позволяет создать
// «точку сохранения» и откатиться только до неё, оставив предыдущие
// операции в силе.
//
//   BEGIN;
//   INSERT INTO orders (user_id, amount) VALUES (1, 100);  -- OK
//   SAVEPOINT after_first_order;
//   INSERT INTO orders (user_id, amount) VALUES (1, 200);  -- попробуем
//   -- что-то пошло не так...
//   ROLLBACK TO SAVEPOINT after_first_order;               -- откат только второго INSERT
//   -- первый INSERT всё ещё в силе!
//   COMMIT;  -- только первый заказ попадёт в БД
//
// Когда использовать:
//   - Обработка батча: часть записей некорректна → пропустить их, сохранить остальные
//   - Вложенные операции: внешняя транзакция не должна страдать из-за внутренней ошибки
//
// Реализуй:
//
//   func InsertBatch(ctx, pool, items []Item) (inserted, skipped int, err error)
//     - BEGIN
//     - для каждого item:
//         SAVEPOINT sp_N
//         INSERT INTO users (name, email) VALUES (...)
//         если ошибка (например, дубликат email) → ROLLBACK TO SAVEPOINT sp_N → skipped++
//         иначе → RELEASE SAVEPOINT sp_N → inserted++
//     - COMMIT
//     - возвращает сколько вставлено и сколько пропущено
//
// Запуск:
//   go mod tidy && go test -v ./...

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Item struct {
	Name  string
	Email string
}

func dsn() string {
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		return v
	}
	return "postgres://postgres:postgres@localhost:5433/postgres?sslmode=disable"
}

// TODO: реализуй InsertBatch - INSERT с savepoint на каждый элемент
func InsertBatch(ctx context.Context, pool *pgxpool.Pool, items []Item) (inserted, skipped int, err error) {
	// Начинаем транзакцию
	// Все операции будут либо выполнены, либо откачены как целое
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	// Гарантированный откат в случае ошибки
	// Если в конце вызовем tx.Commit() — Rollback не сделает ничего
	defer tx.Rollback(ctx)

	for i, item := range items {
		// Создаём уникальное имя для savepoint
		// Например: sp_0, sp_1, sp_2...
		sp := fmt.Sprintf("sp_%d", i)
		// Устанавливаем savepoint
		// С этого момента можно откатиться именно до этой точки
		tx.Exec(ctx, "SAVEPOINT "+sp)
		// Пытаемся вставить запись
		_, insertErr := tx.Exec(ctx,
			"INSERT INTO users (name, email) VALUES ($1, $2)",
			item.Name, item.Email,
		)
		// Если ошибка — откатываемся до savepoint
		if insertErr != nil {
			tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+sp)
			skipped++
		} else {
			// Закрепляем savepoint
			// Теперь нельзя откатиться до этой точки
			tx.Exec(ctx, "RELEASE SAVEPOINT "+sp)
			inserted++ // Увеличиваем счётчик успешных
		}
	}
	// Фиксируем все успешные изменения
	return inserted, skipped, tx.Commit(ctx)
}

func main() {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		log.Fatalf("connect: %v\nЗапусти: docker compose up -d", err)
	}
	defer pool.Close()

	// Предзаполним уже существующий email для симуляции конфликта
	pool.Exec(ctx, "INSERT INTO users (name, email) VALUES ('Existing', 'dup@example.com') ON CONFLICT DO NOTHING")

	items := []Item{
		{Name: "Alice", Email: "alice-sp@example.com"}, // OK
		{Name: "Duplicate", Email: "dup@example.com"},  // КОНФЛИКТ - пропустим
		{Name: "Bob", Email: "bob-sp@example.com"},     // OK
		{Name: "BadEmail", Email: "not-valid"},         // может проходить без доп. constraint
		{Name: "Carol", Email: "carol-sp@example.com"}, // OK
	}

	inserted, skipped, err := InsertBatch(ctx, pool, items)
	if err != nil {
		log.Fatalf("InsertBatch: %v", err)
	}

	fmt.Printf("Вставлено: %d, Пропущено: %d\n", inserted, skipped)
	fmt.Println("Благодаря savepoint: ошибка одного INSERT не откатила весь батч!")

	// Cleanup
	pool.Exec(ctx, "DELETE FROM users WHERE email LIKE '%-sp@example.com' OR email = 'dup@example.com'")
}
