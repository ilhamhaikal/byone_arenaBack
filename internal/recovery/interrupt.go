// Package recovery berisi logika deteksi & penanganan sesi yang terputus
// karena listrik mati / TV-server offline.
package recovery

import (
	"time"

	"byone-arena/pkg/spname"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ShouldInterrupt menentukan apakah sesi expired harus ditandai interrupted
// (bukan completed biasa). Indikasinya: heartbeat TV terakhir terjadi jauh
// sebelum jadwal sesi selesai.
func ShouldInterrupt(db *gorm.DB, sessionID uuid.UUID) bool {
	var row struct {
		EndScheduledAt *time.Time `gorm:"column:end_scheduled_at"`
		LastSeenAt     *time.Time `gorm:"column:last_seen_at"`
	}
	err := db.Table("sessions s").
		Select("s.end_scheduled_at, c.last_seen_at").
		Joins("JOIN consoles c ON c.id = s.console_id").
		Where("s.id = ?", sessionID).
		Scan(&row).Error
	if err != nil || row.EndScheduledAt == nil || row.LastSeenAt == nil {
		return false
	}
	// Toleransi 2 menit: heartbeat normal tiap ~10 detik.
	return row.LastSeenAt.Before(row.EndScheduledAt.Add(-2 * time.Minute))
}

// Interrupt menandai sesi sebagai interrupted dan menyimpan sisa menit.
// Mengembalikan sisa menit yang bisa dipulihkan.
func Interrupt(db *gorm.DB, sessionID uuid.UUID, reason string) (int, error) {
	var row struct {
		RemainingMinutes int `gorm:"column:remaining_minutes"`
	}
	err := db.Raw(
		"SELECT * FROM "+spname.Ident("InterruptSession")+"(?::uuid, ?::text)",
		sessionID, reason,
	).Scan(&row).Error
	if err != nil {
		return 0, err
	}
	return row.RemainingMinutes, nil
}
