// Package janitor menjalankan tugas pembersihan berkala.
package janitor

import (
	"context"
	"fmt"
	"time"

	"byone-arena/pkg/spname"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// StartStaleSessionWatcher menutup sesi yang terlupakan:
// sudah melewati jadwal > 6 jam DAN konsolnya tidak mengirim heartbeat
// > 30 menit. Sesi ditutup tanpa tagihan kelebihan waktu.
//
// Ini mencegah kasus TV/aplikasi mati semalaman lalu ketika diakhiri
// muncul tagihan kelebihan waktu puluhan jam.
func StartStaleSessionWatcher(ctx context.Context, db *gorm.DB, log *zap.Logger) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		run := func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("janitor sesi panic", zap.Any("panic", r))
				}
			}()
			closeStaleSessions(db, log)
		}

		run() // sekali saat start
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}

func closeStaleSessions(db *gorm.DB, log *zap.Logger) {
	type row struct {
		ID uuid.UUID `gorm:"column:id"`
	}

	var rows []row
	err := db.Raw(`
		SELECT s.id
		  FROM sessions s
		  JOIN consoles c ON c.id = s.console_id
		 WHERE s.status = 'active'
		   AND s.end_scheduled_at IS NOT NULL
		   AND s.end_scheduled_at < NOW() - INTERVAL '6 hours'
		   AND (c.last_seen_at IS NULL
		        OR c.last_seen_at < NOW() - INTERVAL '30 minutes')
		 LIMIT 100`).Scan(&rows).Error
	if err != nil {
		log.Warn("janitor: gagal membaca sesi terlantar", zap.Error(err))
		return
	}

	for _, r := range rows {
		sp := fmt.Sprintf("SELECT %s(?)", spname.Ident("ForceCloseSession"))
		if err := db.Exec(sp, r.ID).Error; err != nil {
			log.Warn("janitor: gagal menutup sesi",
				zap.String("session", r.ID.String()), zap.Error(err))
			continue
		}
		log.Info("janitor: sesi ditutup otomatis (tidak ada aktivitas)",
			zap.String("session", r.ID.String()))
	}
}
