-- Инициализация БД для всех задач
-- Этот файл выполняется при первом запуске контейнера

-- Расширения
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_stat_statements";

-- ───────────────────────────────────────────────
-- 01-basics / task02 - CRUD
-- ───────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS users (
    id         SERIAL PRIMARY KEY,
    name       TEXT        NOT NULL,
    email      TEXT        NOT NULL UNIQUE,
    age        INT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ───────────────────────────────────────────────
-- 02-transactions - Банковские счета
-- ───────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS accounts (
    id      SERIAL PRIMARY KEY,
    owner   TEXT           NOT NULL,
    balance NUMERIC(15, 2) NOT NULL CHECK (balance >= 0)
);

-- ───────────────────────────────────────────────
-- 03-isolation - Уровни изоляции
-- ───────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS counters (
    name  TEXT PRIMARY KEY,
    value INT  NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS inventory (
    id       SERIAL PRIMARY KEY,
    product  TEXT NOT NULL,
    quantity INT  NOT NULL CHECK (quantity >= 0)
);

CREATE TABLE IF NOT EXISTS on_call (
    id     SERIAL PRIMARY KEY,
    doctor TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true
);

-- ───────────────────────────────────────────────
-- 04-indexes - Индексы и производительность
-- ───────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS orders (
    id         SERIAL PRIMARY KEY,
    user_id    INT            NOT NULL,
    status     TEXT           NOT NULL DEFAULT 'pending',
    amount     NUMERIC(15, 2) NOT NULL,
    metadata   JSONB,
    created_at TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- ───────────────────────────────────────────────
-- 05-advanced - LISTEN/NOTIFY, Advisory Locks, SKIP LOCKED
-- ───────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS jobs (
    id         SERIAL PRIMARY KEY,
    payload    TEXT        NOT NULL,
    status     TEXT        NOT NULL DEFAULT 'pending',  -- pending | processing | done | failed
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Тригер для NOTIFY при INSERT в jobs
CREATE OR REPLACE FUNCTION notify_new_job()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('new_job', NEW.id::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_notify_new_job ON jobs;
CREATE TRIGGER trg_notify_new_job
    AFTER INSERT ON jobs
    FOR EACH ROW EXECUTE FUNCTION notify_new_job();
