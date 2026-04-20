// ============================================================
// Задача: LISTEN/NOTIFY - push-уведомления из PostgreSQL  🟡 Middle
// ============================================================
//
// LISTEN/NOTIFY - встроенный pub/sub в PostgreSQL.
// Позволяет одному соединению отправлять события, другим - получать их.
//
// Отправка уведомления:
//   NOTIFY channel_name, 'payload'
//   или через: SELECT pg_notify('channel_name', 'payload')
//
// Подписка:
//   LISTEN channel_name
//   Уведомления приходят асинхронно через то же соединение.
//
// В pgx: conn.WaitForNotification(ctx) - ожидает следующее уведомление.
//
// Применение:
//   - Инвалидация кеша: INSERT → NOTIFY → все сервисы сбрасывают кеш
//   - Реактивные обновления: INSERT в jobs → NOTIFY → воркер просыпается
//   - WebSocket: БД → NOTIFY → Go → WebSocket → браузер
//
// В init.sql уже настроен тригер:
//   INSERT INTO jobs → автоматически вызывает pg_notify('new_job', id)
//
// ВАЖНО: LISTEN работает только через *pgx.Conn (не через pool!).
// Пул не гарантирует что слушаешь то же соединение что и получаешь.
//
// Реализуй:
//
//   func Listen(ctx, connStr, channel string, handler func(payload string)) error
//     - pgx.Connect(ctx, connStr)
//     - conn.Exec(ctx, "LISTEN "+channel)
//     - loop: conn.WaitForNotification(ctx) → handler(notif.Payload)
//     - при ctx.Done() → return nil
//
//   func Notify(ctx, pool, channel, payload string) error
//     - pool.Exec(ctx, "SELECT pg_notify($1, $2)", channel, payload)
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

func dsn() string {
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		return v
	}
	return "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
}

// TODO: реализуй Listen - подписка на канал через dedicated соединение
func Listen(ctx context.Context, connStr, channel string, handler func(payload string)) error {
	// conn, err := pgx.Connect(ctx, connStr)
	// if err != nil { return fmt.Errorf("connect: %w", err) }
	// defer conn.Close(ctx)
	//
	// if _, err := conn.Exec(ctx, "LISTEN "+channel); err != nil {
	//     return fmt.Errorf("LISTEN: %w", err)
	// }
	//
	// for {
	//     notif, err := conn.WaitForNotification(ctx)
	//     if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
	//         return nil
	//     }
	//     if err != nil { return err }
	//     handler(notif.Payload)
	// }
	_ = pgx.Connect
	_ = errors.Is
	return nil
}

// TODO: реализуй Notify - отправка уведомления через пул
func Notify(ctx context.Context, pool *pgxpool.Pool, channel, payload string) error {
	// _, err := pool.Exec(ctx, "SELECT pg_notify($1, $2)", channel, payload)
	// return err
	return nil
}

func main() {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	channel := "demo-channel"

	// Запускаем слушателя
	received := make(chan string, 10)
	listenCtx, listenCancel := context.WithCancel(ctx)
	defer listenCancel()

	go func() {
		err := Listen(listenCtx, dsn(), channel, func(payload string) {
			fmt.Printf("  [NOTIFY] получено: %s\n", payload)
			received <- payload
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("Listen: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond) // дать время подключиться

	// Отправляем уведомления
	for i := range 3 {
		payload := fmt.Sprintf("event-%d", i)
		fmt.Printf("Отправляем NOTIFY: %s\n", payload)
		if err := Notify(ctx, pool, channel, payload); err != nil {
			log.Printf("Notify: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Тестируем тригер на таблице jobs
	fmt.Println("\nINSERT в jobs → тригер → NOTIFY 'new_job'")
	go func() {
		err := Listen(listenCtx, dsn(), "new_job", func(payload string) {
			fmt.Printf("  [new_job] job id=%s\n", payload)
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("Listen new_job: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	pool.Exec(ctx, "INSERT INTO jobs (payload) VALUES ('test-job')")

	// Дождёмся уведомлений
	timeout := time.After(2 * time.Second)
	for count := 0; count < 3; {
		select {
		case <-received:
			count++
		case <-timeout:
			fmt.Printf("Получено %d из 3 уведомлений\n", count)
			return
		}
	}

	listenCancel()
	fmt.Println("\nВсе уведомления получены!")
}
