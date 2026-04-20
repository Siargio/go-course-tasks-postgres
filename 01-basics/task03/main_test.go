package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

func testDB(t *testing.T) *sqlx.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	db, err := connect(ctx)
	if err != nil {
		t.Skipf("PostgreSQL недоступен - запусти: docker compose up -d\nОшибка: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func uniqueEmail(t *testing.T) string {
	return fmt.Sprintf("test-%s-%d@example.com", t.Name(), time.Now().UnixNano())
}

func TestCreateUserNamed(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	id, err := CreateUserNamed(ctx, db, UserInput{Name: "TestNamed", Email: uniqueEmail(t), Age: 20})
	if err != nil {
		t.Fatalf("CreateUserNamed: %v", err)
	}
	if id <= 0 {
		t.Errorf("id = %d, хотим > 0", id)
	}
	t.Cleanup(func() { db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", id) })
}

func TestGetUserByID(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	id, err := CreateUserNamed(ctx, db, UserInput{Name: "GetByID", Email: uniqueEmail(t), Age: 25})
	if err != nil {
		t.Fatalf("CreateUserNamed: %v", err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", id) })

	user, err := GetUserByID(ctx, db, id)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if user == nil {
		t.Fatal("GetUserByID вернул nil - должен вернуть пользователя")
	}
	if user.Name != "GetByID" {
		t.Errorf("Name = %q, хотим %q", user.Name, "GetByID")
	}
}

func TestGetUserByIDNotFound(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	user, err := GetUserByID(ctx, db, -999999)
	if err != nil {
		t.Fatalf("GetUserByID(несуществующий) должен вернуть nil, nil: %v", err)
	}
	if user != nil {
		t.Error("GetUserByID(несуществующий) должен вернуть nil")
	}
}

func TestListUsersByIDs(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	id1, _ := CreateUserNamed(ctx, db, UserInput{Name: "IN1", Email: uniqueEmail(t), Age: 1})
	id2, _ := CreateUserNamed(ctx, db, UserInput{Name: "IN2", Email: uniqueEmail(t), Age: 2})
	t.Cleanup(func() {
		db.ExecContext(ctx, "DELETE FROM users WHERE id IN ($1, $2)", id1, id2)
	})

	users, err := ListUsersByIDs(ctx, db, []int64{id1, id2})
	if err != nil {
		t.Fatalf("ListUsersByIDs: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("ListUsersByIDs вернул %d, хотим 2", len(users))
	}
}

func TestListUsersByIDsEmpty(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	users, err := ListUsersByIDs(ctx, db, nil)
	if err != nil {
		t.Fatalf("ListUsersByIDs(nil): %v", err)
	}
	if users != nil && len(users) != 0 {
		t.Errorf("ListUsersByIDs(nil) должен вернуть пустой результат, got %d", len(users))
	}
}
