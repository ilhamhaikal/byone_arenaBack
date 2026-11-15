package config

import (
	"fmt"
	"os"
	"strings"
)

// ValidateRuntime mencegah kekeliruan environment development vs production.
//
// Aturan:
//   - APP_ENV=production dilarang dijalankan dari workspace development
//     (direktori di bawah /home). Produksi hanya dari /opt/byone-arena.
//   - APP_ENV=production wajib memakai database berakhiran _prod.
//   - Mode selain production tidak boleh memakai database produksi.
func (c *Config) ValidateRuntime() error {
	cwd, _ := os.Getwd()

	if c.AppEnv == "production" {
		if strings.HasPrefix(cwd, "/home/") {
			return fmt.Errorf(
				"APP_ENV=production tidak diizinkan dari workspace development (%s). "+
					"Gunakan APP_ENV=development (lihat scripts/run-dev.sh)", cwd)
		}
		if !strings.HasSuffix(c.DBName, "_prod") {
			return fmt.Errorf(
				"APP_ENV=production tetapi DB_NAME=%q tidak berakhiran _prod", c.DBName)
		}
		return nil
	}

	if strings.HasSuffix(c.DBName, "_prod") {
		return fmt.Errorf(
			"mode %s tidak boleh memakai database produksi %q", c.AppEnv, c.DBName)
	}
	return nil
}
