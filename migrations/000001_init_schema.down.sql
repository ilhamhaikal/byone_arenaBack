-- Migration: 000001_init_schema.down.sql
-- Rollback: Hapus semua tabel yang dibuat

DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS customers;
DROP TABLE IF EXISTS consoles;
DROP TABLE IF EXISTS users;
DROP EXTENSION IF EXISTS "uuid-ossp";
