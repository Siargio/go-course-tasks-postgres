# Go PostgreSQL Tasks

Задачи по работе с PostgreSQL - от подключения и CRUD до уровней изоляции, блокировок и продвинутых фич.  
Каждая задача - отдельный Go-модуль с подробным описанием и TODO-стабами.

Уровни сложности: 🟢 Junior · 🟡 Middle · 🔴 Senior

---

## Структура

| # | Раздел | Тема | Задач |
|---|--------|------|-------|
| 01 | [basics](./01-basics/) | Подключение · CRUD · sqlx · Миграции | 3 |
| 02 | [transactions](./02-transactions/) | BEGIN/COMMIT · Savepoint · Nested | 3 |
| 03 | [isolation](./03-isolation/) | Read Committed · Repeatable Read · Serializable | 3 |
| 04 | [indexes](./04-indexes/) | B-tree · Partial · EXPLAIN ANALYZE | 2 |
| 05 | [advanced](./05-advanced/) | LISTEN/NOTIFY · Advisory Lock · SKIP LOCKED | 3 |

**Итого: 14 задач**

---

## 01 · Basics

| Задача | Описание | Уровень |
|--------|----------|---------|
| [task01 - Connect & Ping](./01-basics/task01/) | DSN, pgxpool, проверка соединения, graceful close | 🟢 |
| [task02 - CRUD](./01-basics/task02/) | INSERT / SELECT / UPDATE / DELETE с `pgx` | 🟢 |
| [task03 - sqlx & Named Queries](./01-basics/task03/) | Сканирование в struct, `:name` параметры, `sqlx.In` | 🟡 |

## 02 · Transactions

| Задача | Описание | Уровень |
|--------|----------|---------|
| [task01 - Begin/Commit/Rollback](./02-transactions/task01/) | Перевод денег: атомарность UPDATE + проверка баланса | 🟡 |
| [task02 - Savepoints](./02-transactions/task02/) | Частичный откат внутри транзакции через SAVEPOINT | 🟡 |
| [task03 - Retry on Serialization Error](./02-transactions/task03/) | Автоматический retry при `40001 serialization_failure` | 🔴 |

## 03 · Isolation Levels

| Задача | Описание | Уровень |
|--------|----------|---------|
| [task01 - Dirty / Non-repeatable Read](./03-isolation/task01/) | Воспроизвести и исправить: Read Committed vs Repeatable Read | 🟡 |
| [task02 - Phantom Read](./03-isolation/task02/) | Воспроизвести фантомное чтение и устранить через Serializable | 🟡 |
| [task03 - Write Skew](./03-isolation/task03/) | Аномалия Write Skew - почему Serializable необходим | 🔴 |

## 04 · Indexes

| Задача | Описание | Уровень |
|--------|----------|---------|
| [task01 - B-tree & Partial Index](./04-indexes/task01/) | Создать индексы, измерить ускорение через EXPLAIN ANALYZE | 🟡 |
| [task02 - Index Types](./04-indexes/task02/) | GIN для JSONB, GiST для range, выбор стратегии | 🔴 |

## 05 · Advanced

| Задача | Описание | Уровень |
|--------|----------|---------|
| [task01 - LISTEN/NOTIFY](./05-advanced/task01/) | Уведомления через Postgres: реактивное обновление кеша | 🟡 |
| [task02 - Advisory Locks](./05-advanced/task02/) | Координация воркеров без отдельной таблицы блокировок | 🔴 |
| [task03 - SKIP LOCKED](./05-advanced/task03/) | Очередь задач без дублирования: SELECT FOR UPDATE SKIP LOCKED | 🔴 |

---

## Быстрый старт

```bash
# 1. Поднять PostgreSQL
docker compose up -d

# 2. Перейти в задачу
cd 01-basics/task01
go mod tidy

# 3. Запустить тесты
go test -v ./...

# 4. Или запустить пример
go run main.go
```

> Тесты автоматически пропускаются (`t.Skip`) если Postgres недоступен.

---

## Что такое уровни изоляции (кратко)

```
READ UNCOMMITTED  - видим незакоммиченные изменения (в PG не реализован)
READ COMMITTED    - видим только закоммиченные  (по умолчанию в PG)
REPEATABLE READ   - повторное чтение той же строки даёт тот же результат
SERIALIZABLE      - транзакции как будто выполняются строго последовательно
```

Аномалии которые устраняются повышением уровня:

| Аномалия | Read Committed | Repeatable Read | Serializable |
|----------|----------------|-----------------|--------------|
| Dirty Read | ✅ нет | ✅ нет | ✅ нет |
| Non-repeatable Read | ❌ возможна | ✅ нет | ✅ нет |
| Phantom Read | ❌ возможна | ⚠️ частично | ✅ нет |
| Write Skew | ❌ возможна | ❌ возможна | ✅ нет |

---

## Переменные окружения

```bash
POSTGRES_DSN=postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable
```

Или переопределить адрес:
```bash
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=postgres
```
