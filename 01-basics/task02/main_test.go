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
		t.Skipf("PostgreSQL недоступен - запусти: docker compose up -d\nОшибка: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("PostgreSQL ping failed: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func uniqueEmail(t *testing.T) string {
	return fmt.Sprintf("test-%s-%d@example.com", t.Name(), time.Now().UnixNano())
}

func TestCreateUser(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	id, err := CreateUser(ctx, pool, "TestUser", uniqueEmail(t), 25)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if id <= 0 {
		t.Errorf("CreateUser вернул id=%d, ожидаем > 0", id)
	}
	t.Cleanup(func() { DeleteUser(ctx, pool, id) })
}

func TestGetUserExists(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	id, err := CreateUser(ctx, pool, "GetTest", uniqueEmail(t), 30)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	t.Cleanup(func() { DeleteUser(ctx, pool, id) })

	user, err := GetUser(ctx, pool, id)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user == nil {
		t.Fatal("GetUser вернул nil для существующего пользователя")
	}
	if user.ID != id {
		t.Errorf("user.ID = %d, хотим %d", user.ID, id)
	}
	if user.Name != "GetTest" {
		t.Errorf("user.Name = %q, хотим %q", user.Name, "GetTest")
	}
}

func TestGetUserNotFound(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	user, err := GetUser(ctx, pool, -999999)
	if err != nil {
		t.Fatalf("GetUser(несуществующий) должен вернуть nil, nil - получили ошибку: %v", err)
	}
	if user != nil {
		t.Error("GetUser(несуществующий) должен вернуть nil")
	}
}

func TestUpdateUser(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	id, err := CreateUser(ctx, pool, "Before", uniqueEmail(t), 20)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	t.Cleanup(func() { DeleteUser(ctx, pool, id) })

	if err := UpdateUser(ctx, pool, id, "After"); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}

	user, _ := GetUser(ctx, pool, id)
	if user == nil {
		t.Fatal("после обновления пользователь не найден")
	}
	if user.Name != "After" {
		t.Errorf("имя после обновления: %q, хотим %q", user.Name, "After")
	}
}

func TestDeleteUser(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	id, err := CreateUser(ctx, pool, "ToDelete", uniqueEmail(t), 18)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if err := DeleteUser(ctx, pool, id); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	user, err := GetUser(ctx, pool, id)
	if err != nil {
		t.Fatalf("GetUser после Delete: %v", err)
	}
	if user != nil {
		t.Error("после DeleteUser пользователь всё ещё найден")
	}
}

func TestListUsers(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	id1, _ := CreateUser(ctx, pool, "List1", uniqueEmail(t), 1)
	id2, _ := CreateUser(ctx, pool, "List2", uniqueEmail(t), 2)
	t.Cleanup(func() {
		DeleteUser(ctx, pool, id1)
		DeleteUser(ctx, pool, id2)
	})

	users, err := ListUsers(ctx, pool)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) < 2 {
		t.Errorf("ListUsers вернул %d, ожидаем >= 2", len(users))
	}
}
