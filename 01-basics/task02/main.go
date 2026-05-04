// ============================================================
// Задача: CRUD - INSERT / SELECT / UPDATE / DELETE  🟢 Junior
// ============================================================
//
// Два способа работы с pgx:
//
//   pool.Exec(ctx, sql, args...)
//     - для INSERT/UPDATE/DELETE где результат не нужен
//     - возвращает CommandTag (сколько строк затронуто)
//
//   pool.QueryRow(ctx, sql, args...)
//     - для SELECT одной строки (или INSERT...RETURNING)
//     - .Scan(&field1, &field2) заполняет переменные
//
//   pool.Query(ctx, sql, args...)
//     - для SELECT нескольких строк
//     - возвращает pgx.Rows, нужно итерироваться и Scan каждую
//
// Параметры запроса:
//   pgx использует $1, $2, ... (НЕ ? как в database/sql)
//   Например: SELECT * FROM users WHERE id = $1 AND name = $2
//
// NULL-able поля:
//   Используй *int, *string или pgtype.Int4, pgtype.Text
//   для полей, которые могут быть NULL в БД.
//
// Реализуй:
//
//   func CreateUser(ctx, pool, name, email string, age int) (int64, error)
//     - INSERT INTO users (name, email, age) VALUES ($1, $2, $3) RETURNING id
//     - возвращает id созданного пользователя
//
//   func GetUser(ctx, pool, id int64) (*User, error)
//     - SELECT * FROM users WHERE id = $1
//     - если не найден - возвращает nil, nil (не ошибку!)
//     - Подсказка: errors.Is(err, pgx.ErrNoRows)
//
//   func UpdateUser(ctx, pool, id int64, name string) error
//     - UPDATE users SET name = $1 WHERE id = $2
//
//   func DeleteUser(ctx, pool, id int64) error
//     - DELETE FROM users WHERE id = $1
//
//   func ListUsers(ctx, pool) ([]User, error)
//     - SELECT * FROM users ORDER BY id
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
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID        int64
	Name      string
	Email     string
	Age       *int
	CreatedAt time.Time
}

func dsn() string {
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		return v
	}
	return "postgres://postgres:postgres@localhost:5433/postgres?sslmode=disable"
}

// TODO: реализуй CreateUser - INSERT RETURNING id
func CreateUser(ctx context.Context, pool *pgxpool.Pool, name, email string, age int) (int64, error) {
	// Подсказка:
	var id int64
	err := pool.QueryRow(ctx,
		"INSERT INTO users (name, email, age) VALUES ($1, $2, $3) RETURNING id",
		name, email, age,
	).Scan(&id)
	return id, err
}

// TODO: реализуй GetUser - SELECT WHERE id
func GetUser(ctx context.Context, pool *pgxpool.Pool, id int64) (*User, error) {
	// Подсказка:
	u := &User{}
	err := pool.QueryRow(ctx,
		"SELECT id, name, email, age, created_at FROM users WHERE id = $1", id,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Age, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	} // не найден - не ошибка
	if err != nil {
		return nil, err
	}
	return u, nil
}

// TODO: реализуй UpdateUser - UPDATE users SET name
func UpdateUser(ctx context.Context, pool *pgxpool.Pool, id int64, name string) error {
	// Подсказка:
	tag, err := pool.Exec(ctx, "UPDATE users SET name = $1 WHERE id = $2", name, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("user %d not found", id)
	}
	return nil
}

// TODO: реализуй DeleteUser - DELETE FROM users
func DeleteUser(ctx context.Context, pool *pgxpool.Pool, id int64) error {
	_, err := pool.Exec(ctx, "DELETE FROM users WHERE id = $1", id)
	return err
}

// TODO: реализуй ListUsers - SELECT * ORDER BY id
func ListUsers(ctx context.Context, pool *pgxpool.Pool) ([]User, error) {
	rows, err := pool.Query(ctx, "SELECT id, name, email, age, created_at FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Age, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func main() {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		log.Fatalf("connect: %v\nЗапусти: docker compose up -d", err)
	}
	defer pool.Close()

	// CREATE
	age := 30
	id, err := CreateUser(ctx, pool, "Alice", fmt.Sprintf("alice-%d@example.com", time.Now().UnixNano()), age)
	if err != nil {
		log.Fatalf("CreateUser: %v", err)
	}
	fmt.Printf("Создан user id=%d\n", id)

	// READ
	user, err := GetUser(ctx, pool, id)
	if err != nil {
		log.Fatalf("GetUser: %v", err)
	}
	if user == nil {
		log.Fatal("GetUser вернул nil - пользователь не найден")
	}
	fmt.Printf("Получен: %+v\n", user)

	// UPDATE
	if err := UpdateUser(ctx, pool, id, "Alice Updated"); err != nil {
		log.Fatalf("UpdateUser: %v", err)
	}
	fmt.Printf("Обновлено имя на 'Alice Updated'\n")

	// LIST
	users, err := ListUsers(ctx, pool)
	if err != nil {
		log.Fatalf("ListUsers: %v", err)
	}
	fmt.Printf("Всего пользователей: %d\n", len(users))

	// DELETE
	if err := DeleteUser(ctx, pool, id); err != nil {
		log.Fatalf("DeleteUser: %v", err)
	}
	fmt.Printf("Удалён user id=%d\n", id)

	// Проверка удаления
	deleted, _ := GetUser(ctx, pool, id)
	fmt.Printf("После удаления GetUser = %v (должно быть nil)\n", deleted)
}
