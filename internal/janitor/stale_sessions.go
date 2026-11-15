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

type sessionRow struct {
	ID uuid.UUID `gorm:"column:id"`
}

// StartStaleSessionWatcher menjalankan dua pengaman tiap menit:
//
//  1. Auto-end: sesi yang waktu sewanya sudah habis ditutup tepat pada
//     end_scheduled_at, tanpa tagihan tambahan.
//  2. Stale-close: sesi yang telat > 6 jam dan TV-nya tidak mengirim heartbeat
//     > 30 menit ditutup tanpa tagihan (fallback bila auto-end gagal).
func StartStaleSessionWatcher(ctx context.Context, db *gorm.DB, log *zap.Logger) {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		run := func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("janitor panic", zap.Any("panic", r))
				}
			}()
			autoEndExpiredSessions(db, log)
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

// autoEndExpiredSessions menutup sesi yang sudah melewati jadwal selesai.
func autoEndExpiredSessions(db *gorm.DB, log *zap.Logger) {
	var rows []sessionRow
	err := db.Raw(`
		SELECT id
		  FROM sessions
		 WHERE status = 'active'
		   AND end_scheduled_at IS NOT NULL
		   AND end_scheduled_at <= NOW()
		 LIMIT 200`).Scan(&rows).Error
	if err != nil {
		log.Warn("janitor: gagal membaca sesi kedaluwarsa", zap.Error(err))
		return
	}

	for _, r := range rows {
		sp := fmt.Sprintf("SELECT %s(?)", spname.Ident("AutoEndSession"))
		if err := db.Exec(sp, r.ID).Error; err != nil {
			log.Warn("janitor: gagal auto-end sesi",
				zap.String("session", r.ID.String()), zap.Error(err))
			continue
		}
		log.Info("janitor: sesi diakhiri otomatis (waktu habis)",
			zap.String("session", r.ID.String()))
	}
}

// closeStaleSessions menutup sesi yang terlantar (TV mati / aplikasi tidak
// berjalan) tanpa tagihan tambahan.
func closeStaleSessions(db *gorm.DB, log *zap.Logger) {
	var rows []sessionRow
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
