-- Rollback seed akun owner (hanya menghapus akun default, bukan akun lain)
DELETE FROM users WHERE username = 'owner';
