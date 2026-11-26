package entity

import (
	"math"
	"time"

	"github.com/google/uuid"
)

// SessionStatus mendefinisikan status sesi rental
type SessionStatus string

const (
	SessionStatusActive    SessionStatus = "active"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusCancelled SessionStatus = "cancelled"
	// SessionStatusInterrupted: sesi terputus (mis. listrik mati) dan sisa
	// waktunya bisa dipulihkan lewat voucher GRATIS.
	SessionStatusInterrupted SessionStatus = "interrupted"
	// SessionStatusRecovered: sesi interrupted yang sudah dipulihkan.
	SessionStatusRecovered SessionStatus = "recovered"
)

// Session merepresentasikan satu sesi penyewaan konsol / TV Android
type Session struct {
	ID         uuid.UUID  `json:"id"                     gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	ConsoleID  uuid.UUID  `json:"consoleId"              gorm:"type:uuid;not null;index"`
	CustomerID *uuid.UUID `json:"customerId,omitempty"   gorm:"type:uuid;index"` // nullable, walk-in
	StartTime  time.Time  `json:"startTime"              gorm:"not null"`
	EndTime    *time.Time `json:"endTime,omitempty"`
	// BookedDurationMinutes adalah durasi yang dipesan di awal (contoh: 120 menit = 2 jam)
	BookedDurationMinutes int `json:"bookedDurationMinutes"  gorm:"not null;default:0"`
	// EndScheduledAt adalah waktu selesai yang direncanakan (StartTime + BookedDurationMinutes)
	EndScheduledAt  *time.Time    `json:"endScheduledAt,omitempty"`
	DurationMinutes int           `json:"durationMinutes"        gorm:"not null;default:0"`
	TotalPrice      float64       `json:"totalPrice"             gorm:"not null;default:0;type:numeric(10,2)"`
	Status          SessionStatus `json:"status"                 gorm:"not null;default:'active';size:20;index"`
	Notes           string        `json:"notes,omitempty"        gorm:"type:text"`
	// Pemulihan sesi terputus (listrik mati).
	InterruptedAt            *time.Time `json:"interruptedAt,omitempty"         gorm:"column:interrupted_at"`
	RemainingRecoveryMinutes int        `json:"remainingMinutes"                gorm:"column:remaining_minutes;not null;default:0"`
	InterruptedReason        string     `json:"interruptedReason,omitempty"     gorm:"column:interrupted_reason;type:text"`
	// SavedTimeExpiresAt: batas pemulihan sisa waktu non-member (24 jam).
	// NULL = tanpa kedaluwarsa (mis. sesi member / bank waktu).
	SavedTimeExpiresAt     *time.Time `json:"savedTimeExpiresAt,omitempty"     gorm:"column:saved_time_expires_at"`
	RecoveredBySessionID   *uuid.UUID `json:"recoveredBySessionId,omitempty"  gorm:"type:uuid"`
	RecoveredFromSessionID *uuid.UUID `json:"recoveredFromSessionId,omitempty" gorm:"type:uuid"`
	// Transparansi Bank Waktu & Poin Loyalitas (khusus member)
	// PaidDurationMinutes adalah durasi yang benar-benar DIBAYAR (dasar hitung harga & poin).
	// BookedDurationMinutes (di atas) adalah TOTAL menit bermain termasuk bonus dari bank waktu/poin.
	PaidDurationMinutes    int `json:"paidDurationMinutes"     gorm:"not null;default:0"`
	TimeBankMinutesUsed    int `json:"timeBankMinutesUsed"     gorm:"not null;default:0"` // menit bonus dari saldo bank waktu member
	BonusMinutesFromPoints int `json:"bonusMinutesFromPoints"  gorm:"not null;default:0"` // menit bonus dari penukaran poin
	PointsRedeemed         int `json:"pointsRedeemed"          gorm:"not null;default:0"` // poin yang ditukar pada sesi ini
	PointsEarned           int `json:"pointsEarned"            gorm:"not null;default:0"` // poin yang didapat dari sesi ini
	// Bonus waktu gratis dari promo (mis. weekend 30 menit). Tidak menambah
	// tagihan; hanya menambah total menit bermain (booked_duration_minutes).
	PromoBonusMinutes int `json:"promoBonusMinutes" gorm:"not null;default:0"`
	PromoApplyCount   int `json:"promoApplyCount"   gorm:"not null;default:0"`
	// IsPackage menandai sesi dari paket rental (mis. begadang 21:00-06:00).
	// Sesi paket tidak boleh di-extend; waktunya mengikuti jadwal paket.
	IsPackage      bool       `json:"isPackage"         gorm:"not null;default:false;index"`
	PromoRuleID    *uuid.UUID `json:"promoRuleId,omitempty" gorm:"type:uuid"`
	PromoAppliedAt *time.Time `json:"promoAppliedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"              gorm:"autoCreateTime"`
	UpdatedAt      time.Time  `json:"updatedAt"              gorm:"autoUpdateTime"`

	// Relasi (join data)
	Console  *Console  `json:"console,omitempty"  gorm:"foreignKey:ConsoleID"`
	Customer *Customer `json:"customer,omitempty" gorm:"foreignKey:CustomerID"`

	// Transient (bukan kolom DB): jumlah menit yang baru disimpan ke bank waktu
	// saat sesi diakhiri. Dipakai UI untuk menampilkan info ke kasir/member.
	TimeBankEarnedMinutes int `json:"timeBankEarnedMinutes,omitempty" gorm:"-"`
}

func (Session) TableName() string { return "sessions" }

// ApplySetupGrace menggeser jam mulai sesi (dan jadwal selesainya) sebesar
// beberapa menit. Dipakai untuk memberi waktu persiapan konsol di AWAL sesi:
// durasi berbayar (end - start) tetap, tetapi jam sesi baru berjalan setelah
// waktu setup selesai.
//
// Mengembalikan false bila sesi tidak punya jadwal selesai (open-ended) atau
// menit <= 0.
func (s *Session) ApplySetupGrace(minutes int) bool {
	if minutes <= 0 || s.EndScheduledAt == nil {
		return false
	}
	shift := time.Duration(minutes) * time.Minute
	s.StartTime = s.StartTime.Add(shift)
	end := s.EndScheduledAt.Add(shift)
	s.EndScheduledAt = &end
	return true
}

// CalculateDuration menghitung durasi sesi aktual dalam menit
func (s *Session) CalculateDuration() int {
	if s.EndTime == nil {
		return int(time.Since(s.StartTime).Minutes())
	}
	return int(s.EndTime.Sub(s.StartTime).Minutes())
}

// CalculateTotalPrice menghitung total harga berdasarkan durasi dan harga per jam.
// Dibulatkan ke rupiah utuh (tanpa desimal) — konsisten dengan stored procedure
// pricing (CalculatePrice/EndSession) agar tidak ada selisih fraksional yang
// menyebabkan validasi "uang kurang" padahal nominal sama dengan yang ditampilkan.
func (s *Session) CalculateTotalPrice(pricePerHour float64) float64 {
	durationHours := float64(s.CalculateDuration()) / 60.0
	return math.Round(durationHours * pricePerHour)
}

// RemainingMinutes menghitung sisa menit sesi berdasarkan durasi yang dipesan.
// Mengembalikan -1 jika sesi tidak memiliki durasi pre-book (open-ended).
// Mengembalikan 0 jika sesi sudah melewati jadwal selesai.
func (s *Session) RemainingMinutes() int {
	if s.EndScheduledAt == nil {
		return -1
	}
	rem := int(time.Until(*s.EndScheduledAt).Minutes())
	if rem < 0 {
		return 0
	}
	return rem
}

// RemainingSeconds menghitung sisa DETIK sesi berdasarkan durasi yang dipesan.
// Mengembalikan -1 jika sesi tidak memiliki durasi pre-book (open-ended).
// Mengembalikan 0 jika sesi sudah melewati jadwal selesai.
//
// Berbeda dengan RemainingMinutes yang memakai floor ke menit (sisa 40 detik
// akan menjadi 0 menit), method ini membulatkan KE ATAS (ceil) supaya sesi
// dengan sisa < 60 detik tidak pernah dilaporkan 0. Nilai detik inilah yang
// dipakai client TV sebagai seed countdown clock-sync-safe agar countdown
// presisi hingga detik.
func (s *Session) RemainingSeconds() int {
	if s.EndScheduledAt == nil {
		return -1
	}
	rem := int(math.Ceil(time.Until(*s.EndScheduledAt).Seconds()))
	if rem < 0 {
		return 0
	}
	return rem
}
