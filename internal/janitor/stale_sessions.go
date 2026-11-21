// Package janitor menjalankan tugas pembersihan berkala.
package janitor

import (
	"context"
	"time"

	"byone-arena/internal/recovery"
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
			// Auto-end sesi expired ditangani satu tempat oleh Hub.StartAutoStop
			// (termasuk broadcast realtime). Janitor hanya menangani sesi
			// terlantar yang TV-nya offline.
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

// closeStaleSessions menutup sesi yang terlantar (TV mati / aplikasi tidak
// berjalan) tanpa tagihan tambahan.
func closeStaleSessions(db *gorm.DB, log *zap.Logger) {
	var rows []sessionRow
	err := db.Raw(
		"SELECT * FROM "+spname.Ident("StaleActiveSessions")+"(?)", 100,
	).Scan(&rows).Error
	if err != nil {
		log.Warn("janitor: gagal membaca sesi terlantar", zap.Error(err))
		return
	}

	for _, r := range rows {
		// Sesi terlantar = TV sudah offline lama → simpan sisa waktu,
		// jangan hanguskan. Admin bisa memulihkan dengan voucher GRATIS.
		if _, err := recovery.Interrupt(db, r.ID, "tv_offline"); err != nil {
			log.Warn("janitor: gagal menandai sesi terputus",
				zap.String("session", r.ID.String()), zap.Error(err))
			continue
		}
		markConsoleScreenOff(db, r.ID)
		log.Info("janitor: sesi ditandai terputus (TV offline)",
			zap.String("session", r.ID.String()))
	}
}

// markConsoleScreenOff memastikan TV kembali ke screensaver dan keluar dari
// mode live manual setelah sesi ditutup otomatis.
func markConsoleScreenOff(db *gorm.DB, sessionID uuid.UUID) {
	db.Exec(
		"UPDATE consoles SET status='available', screen_status='off', manual_live=false, updated_at=NOW() "+
			"WHERE id = (SELECT console_id FROM sessions WHERE id = ?)",
		sessionID,
	)
}
