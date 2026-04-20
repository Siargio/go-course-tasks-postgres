// ============================================================
// Задача: SKIP LOCKED - очередь задач без дублирования  🔴 Senior
// ============================================================
//
// Проблема: несколько воркеров читают задачи из таблицы jobs.
//
//   Наивный подход (НЕВЕРНЫЙ):
//     SELECT * FROM jobs WHERE status = 'pending' LIMIT 1
//     UPDATE jobs SET status = 'processing' WHERE id = $1
//
//   Проблема: оба воркера могут выбрать ОДНУ задачу до UPDATE.
//   Оба будут её обрабатывать → дублирование.
//
//   SELECT FOR UPDATE - правильнее, но воркеры будут ЖДАТЬ друг друга:
//     Worker1: SELECT FOR UPDATE → получает задачу 1, блокирует её
//     Worker2: SELECT FOR UPDATE → ЖДЁТ пока Worker1 не закончит
//
//   SELECT FOR UPDATE SKIP LOCKED - оптимально:
//     Worker1: SELECT FOR UPDATE SKIP LOCKED → получает задачу 1
//     Worker2: SELECT FOR UPDATE SKIP LOCKED → ПРОПУСКАЕТ задачу 1 (заблокирована)
//                                               → получает задачу 2 сразу, без ожидания
//
// SQL:
//   BEGIN;
//   SELECT id, payload FROM jobs
//   WHERE status = 'pending'
//   ORDER BY id
//   LIMIT 1
//   FOR UPDATE SKIP LOCKED;
//
//   UPDATE jobs SET status = 'processing' WHERE id = $1;
//   COMMIT;
//
// Реализуй:
//
//   type Job struct { ID int64; Payload string }
//
//   func Enqueue(ctx, pool, payload string) (int64, error)
//     - INSERT INTO jobs (payload) VALUES ($1) RETURNING id
//
//   func Dequeue(ctx, pool) (*Job, error)
//     - BEGIN
//     - SELECT id, payload FROM jobs WHERE status='pending' ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED
//     - если нет строк → ROLLBACK, return nil, nil
//     - UPDATE jobs SET status='processing' WHERE id=$1
//     - COMMIT
//     - return &Job{...}
//
//   func Complete(ctx, pool, id int64) error
//     - UPDATE jobs SET status='done' WHERE id=$1
//
//   func Fail(ctx, pool, id int64) error
//     - UPDATE jobs SET status='failed' WHERE id=$1
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
	"sync/atomic"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Job struct {
	ID      int64
	Payload string
}

func dsn() string {
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		return v
	}
	return "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
}

// TODO: реализуй Enqueue
func Enqueue(ctx context.Context, pool *pgxpool.Pool, payload string) (int64, error) {
	// var id int64
	// err := pool.QueryRow(ctx,
	//     "INSERT INTO jobs (payload) VALUES ($1) RETURNING id", payload,
	// ).Scan(&id)
	// return id, err
	return 0, nil
}

// TODO: реализуй Dequeue - атомарное получение задачи с SKIP LOCKED
func Dequeue(ctx context.Context, pool *pgxpool.Pool) (*Job, error) {
	// tx, err := pool.Begin(ctx)
	// if err != nil { return nil, err }
	// defer tx.Rollback(ctx)
	//
	// var job Job
	// err = tx.QueryRow(ctx, `
	//     SELECT id, payload FROM jobs
	//     WHERE status = 'pending'
	//     ORDER BY id
	//     LIMIT 1
	//     FOR UPDATE SKIP LOCKED
	// `).Scan(&job.ID, &job.Payload)
	//
	// if errors.Is(err, pgx.ErrNoRows) { return nil, nil }
	// if err != nil { return nil, err }
	//
	// _, err = tx.Exec(ctx, "UPDATE jobs SET status = 'processing' WHERE id = $1", job.ID)
	// if err != nil { return nil, err }
	//
	// return &job, tx.Commit(ctx)
	_ = pgx.ErrNoRows
	_ = errors.Is
	return nil, nil
}

// TODO: реализуй Complete
func Complete(ctx context.Context, pool *pgxpool.Pool, id int64) error {
	// _, err := pool.Exec(ctx, "UPDATE jobs SET status = 'done', updated_at = NOW() WHERE id = $1", id)
	// return err
	return nil
}

// TODO: реализуй Fail
func Fail(ctx context.Context, pool *pgxpool.Pool, id int64) error {
	// _, err := pool.Exec(ctx, "UPDATE jobs SET status = 'failed', updated_at = NOW() WHERE id = $1", id)
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

	// Кладём 10 задач в очередь
	fmt.Println("Добавляем 10 задач...")
	var jobIDs []int64
	for i := range 10 {
		id, err := Enqueue(ctx, pool, fmt.Sprintf("task-%d", i))
		if err != nil {
			log.Fatalf("Enqueue: %v", err)
		}
		jobIDs = append(jobIDs, id)
	}

	// 3 параллельных воркера - каждая задача должна обработаться РОВНО ОДИН раз
	var (
		wg        sync.WaitGroup
		processed atomic.Int32
	)

	for w := range 3 {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				job, err := Dequeue(ctx, pool)
				if err != nil {
					log.Printf("worker %d: Dequeue: %v", workerID, err)
					return
				}
				if job == nil {
					return // очередь пуста
				}
				fmt.Printf("  Воркер %d обрабатывает job %d (%s)\n", workerID, job.ID, job.Payload)
				Complete(ctx, pool, job.ID)
				processed.Add(1)
			}
		}(w)
	}

	wg.Wait()
	fmt.Printf("\nОбработано задач: %d из 10\n", processed.Load())
	if processed.Load() == 10 {
		fmt.Println("Все задачи обработаны без дублирования!")
	}

	// Cleanup
	pool.Exec(ctx, "DELETE FROM jobs WHERE id = ANY($1)", jobIDs)
}
