// ============================================================
// Задача: sqlx - сканирование в struct, named queries  🟡 Middle
// ============================================================
//
// sqlx - обёртка над database/sql, добавляющая:
//   - StructScan: сканирование строк в struct по тегам `db:"column_name"`
//   - Named queries: INSERT ... VALUES (:name, :email) вместо $1, $2
//   - sqlx.In: генерация IN ($1, $2, $3, ...) из слайса
//   - Get/Select: удобные обёртки над QueryRow/Query + Scan
//
// Почему sqlx + pgx вместе?
//   sqlx работает с database/sql интерфейсом.
//   pgx тоже предоставляет stdlib-совместимый драйвер: pgxpool → sql.Open.
//   Используем: sqlx.NewDb(sql.Open("pgx", dsn))
//
// Struct tags:
//   type User struct {
//       ID    int64  `db:"id"`
//       Name  string `db:"name"`
//       Email string `db:"email"`
//   }
//   sqlx.Get(db, &user, "SELECT * FROM users WHERE id = $1", id)
//   → автоматически сканирует колонки в поля по тегу `db`
//
// Реализуй:
//
//   func GetUserByID(ctx, db *sqlx.DB, id int64) (*UserRow, error)
//     - db.GetContext(ctx, &user, query, id)
//     - если не найден - errors.Is(err, sql.ErrNoRows) → return nil, nil
//
//   func ListUsersByIDs(ctx, db *sqlx.DB, ids []int64) ([]UserRow, error)
//     - sqlx.In("SELECT * FROM users WHERE id IN (?)", ids)
//     - db.Rebind(query) - конвертирует ? в $1, $2, ... для pgx
//     - db.SelectContext(ctx, &users, query, args...)
//
//   func CreateUserNamed(ctx, db *sqlx.DB, user UserInput) (int64, error)
//     - db.NamedQueryContext с INSERT INTO users (:name, :email, :age)
//     - возвращает id через rows.Scan
//
// Запуск:
//   go mod tidy && go test -v ./...

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type UserRow struct {
	ID        int64      `db:"id"`
	Name      string     `db:"name"`
	Email     string     `db:"email"`
	Age       *int       `db:"age"`
	CreatedAt time.Time  `db:"created_at"`
}

type UserInput struct {
	Name  string `db:"name"`
	Email string `db:"email"`
	Age   int    `db:"age"`
}

func dsn() string {
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		return v
	}
	return "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
}

func connect(ctx context.Context) (*sqlx.DB, error) {
	db, err := sqlx.Open("pgx", dsn())
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// TODO: реализуй GetUserByID - GetContext + struct scan
func GetUserByID(ctx context.Context, db *sqlx.DB, id int64) (*UserRow, error) {
	// var u UserRow
	// err := db.GetContext(ctx, &u, "SELECT id, name, email, age, created_at FROM users WHERE id = $1", id)
	// if errors.Is(err, sql.ErrNoRows) { return nil, nil }
	// if err != nil { return nil, err }
	// return &u, nil
	_ = errors.Is
	_ = sql.ErrNoRows
	return nil, nil
}

// TODO: реализуй ListUsersByIDs - sqlx.In + Rebind + SelectContext
func ListUsersByIDs(ctx context.Context, db *sqlx.DB, ids []int64) ([]UserRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	// query, args, err := sqlx.In("SELECT id, name, email, age, created_at FROM users WHERE id IN (?) ORDER BY id", ids)
	// if err != nil { return nil, err }
	// query = db.Rebind(query)  // ? → $1, $2, ...
	//
	// var users []UserRow
	// err = db.SelectContext(ctx, &users, query, args...)
	// return users, err
	return nil, nil
}

// TODO: реализуй CreateUserNamed - NamedQueryContext
func CreateUserNamed(ctx context.Context, db *sqlx.DB, input UserInput) (int64, error) {
	// rows, err := db.NamedQueryContext(ctx,
	//     "INSERT INTO users (name, email, age) VALUES (:name, :email, :age) RETURNING id",
	//     input,
	// )
	// if err != nil { return 0, err }
	// defer rows.Close()
	// if !rows.Next() { return 0, fmt.Errorf("no rows returned") }
	// var id int64
	// return id, rows.Scan(&id)
	return 0, nil
}

func main() {
	ctx := context.Background()

	db, err := connect(ctx)
	if err != nil {
		log.Fatalf("connect: %v\nЗапусти: docker compose up -d", err)
	}
	defer db.Close()

	// CREATE через named query
	id, err := CreateUserNamed(ctx, db, UserInput{
		Name:  "Bob",
		Email: fmt.Sprintf("bob-%d@example.com", time.Now().UnixNano()),
		Age:   28,
	})
	if err != nil {
		log.Fatalf("CreateUserNamed: %v", err)
	}
	fmt.Printf("Создан user id=%d\n", id)

	// GET через struct scan
	user, err := GetUserByID(ctx, db, id)
	if err != nil {
		log.Fatalf("GetUserByID: %v", err)
	}
	if user == nil {
		log.Fatal("GetUserByID: не найден")
	}
	fmt.Printf("Получен: %+v\n", *user)

	// LIST IN через sqlx.In
	id2, _ := CreateUserNamed(ctx, db, UserInput{
		Name:  "Carol",
		Email: fmt.Sprintf("carol-%d@example.com", time.Now().UnixNano()),
		Age:   32,
	})
	users, err := ListUsersByIDs(ctx, db, []int64{id, id2})
	if err != nil {
		log.Fatalf("ListUsersByIDs: %v", err)
	}
	fmt.Printf("Найдено через IN: %d пользователей\n", len(users))

	// Cleanup
	db.ExecContext(ctx, "DELETE FROM users WHERE id IN ($1, $2)", id, id2)
}
