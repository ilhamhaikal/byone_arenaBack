// Package promo mengatur bonus waktu gratis (mis. akhir pekan).
//
// Alurnya: aturan disimpan di tabel promo_bonus_rules; saat sesi dimulai atau
// saat sesi masih aktif, kasir/admin dapat memilih memakai bonus. Bonus TIDAK
// menambah tagihan: menit bonus hanya menambah booked_duration_minutes
// (= total menit bermain), sedangkan paid_duration_minutes tetap.
package promo

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"byone-arena/internal/domain/entity"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNoActiveRule = errors.New("tidak ada promo bonus yang berlaku")
	ErrNotEligible  = errors.New("sesi belum memenuhi syarat promo")
	ErrLimitReached = errors.New("batas pemakaian bonus untuk sesi ini sudah tercapai")
	ErrNotActive    = errors.New("sesi tidak aktif")
)

var jakarta = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}
	return loc
}()

// Now mengembalikan waktu sekarang dalam zona Asia/Jakarta.
func Now() time.Time { return time.Now().In(jakarta) }

// ParseDays mengubah "6,7" menjadi set hari.
func ParseDays(s string) map[int]bool {
	days := map[int]bool{}
	for _, part := range strings.Split(s, ",") {
		v, err := strconv.Atoi(strings.TrimSpace(part))
		if err == nil && v >= 0 && v <= 6 {
			days[v] = true
		}
	}
	return days
}

// MatchesDay memeriksa apakah waktu t jatuh pada hari yang diatur.
func MatchesDay(rule *entity.PromoBonusRule, t time.Time) bool {
	return ParseDays(rule.DaysOfWeek)[int(t.In(jakarta).Weekday())]
}

// WithinValidity memeriksa rentang tanggal berlaku (inklusif).
func WithinValidity(rule *entity.PromoBonusRule, t time.Time) bool {
	d := t.In(jakarta)
	if rule.ValidFrom != nil && d.Before(rule.ValidFrom.In(jakarta).Truncate(24*time.Hour)) {
		return false
	}
	if rule.ValidTo != nil && d.After(rule.ValidTo.In(jakarta).Add(24*time.Hour)) {
		return false
	}
	return true
}

// Eligible memeriksa syarat minimal menit yang dibayar.
func Eligible(rule *entity.PromoBonusRule, paidMinutes int) bool {
	return rule.BonusMinutes > 0 && paidMinutes >= rule.MinPaidMinutes
}

// ActiveRuleToday mengambil aturan aktif pertama yang berlaku hari ini.
func ActiveRuleToday(db *gorm.DB) (*entity.PromoBonusRule, error) {
	var rules []entity.PromoBonusRule
	if err := db.Where("is_active = TRUE").Order("min_paid_minutes ASC, bonus_minutes DESC").
		Find(&rules).Error; err != nil {
		return nil, err
	}
	now := Now()
	for i := range rules {
		if MatchesDay(&rules[i], now) && WithinValidity(&rules[i], now) {
			return &rules[i], nil
		}
	}
	return nil, ErrNoActiveRule
}

// PaidMinutes mengembalikan menit yang dibayar untuk sebuah sesi.
func PaidMinutes(s *entity.Session) int {
	if s.PaidDurationMinutes > 0 {
		return s.PaidDurationMinutes
	}
	return s.BookedDurationMinutes
}

// Apply menambahkan bonus ke sesi secara atomik.
// Mengembalikan sesi setelah diperbarui.
func Apply(db *gorm.DB, sessionID string) (*entity.Session, *entity.PromoBonusRule, error) {
	var out entity.Session
	var usedRule *entity.PromoBonusRule

	err := db.Transaction(func(tx *gorm.DB) error {
		var s entity.Session
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&s, "id = ?", sessionID).Error; err != nil {
			return err
		}
		if s.Status != entity.SessionStatusActive {
			return ErrNotActive
		}

		paid := PaidMinutes(&s)

		// Kandidat aturan: yang berlaku pada TANGGAL MULAI sesi, agar sesi yang
		// dimulai Sabtu tetap bisa memakai bonus walau baru diaktifkan lewat
		// tengah malam.
		var rules []entity.PromoBonusRule
		if err := tx.Where("is_active = TRUE").Order("min_paid_minutes ASC, bonus_minutes DESC").
			Find(&rules).Error; err != nil {
			return err
		}

		var rule *entity.PromoBonusRule
		for i := range rules {
			if MatchesDay(&rules[i], s.StartTime) &&
				WithinValidity(&rules[i], s.StartTime) &&
				Eligible(&rules[i], paid) {
				rule = &rules[i]
				break
			}
		}
		if rule == nil {
			return ErrNotEligible
		}
		if rule.MaxPerSession > 0 && s.PromoApplyCount >= rule.MaxPerSession {
			return ErrLimitReached
		}

		now := Now()
		newBooked := s.BookedDurationMinutes + rule.BonusMinutes
		var newEnd *time.Time
		if s.EndScheduledAt != nil {
			e := s.EndScheduledAt.Add(time.Duration(rule.BonusMinutes) * time.Minute)
			newEnd = &e
		} else if s.BookedDurationMinutes > 0 {
			e := s.StartTime.Add(time.Duration(newBooked) * time.Minute)
			newEnd = &e
		}

		updates := map[string]any{
			// Akumulasi: pemakaian berulang menambah menit bonus, bukan menimpa.
			"promo_bonus_minutes":     s.PromoBonusMinutes + rule.BonusMinutes,
			"promo_apply_count":       s.PromoApplyCount + 1,
			"promo_rule_id":           rule.ID,
			"promo_applied_at":        now,
			"booked_duration_minutes": newBooked,
			"updated_at":              now,
		}
		if newEnd != nil {
			updates["end_scheduled_at"] = *newEnd
		}
		if err := tx.Model(&entity.Session{}).Where("id = ?", s.ID).Updates(updates).Error; err != nil {
			return err
		}

		usedRule = rule
		return tx.First(&out, "id = ?", s.ID).Error
	})
	if err != nil {
		return nil, nil, err
	}
	return &out, usedRule, nil
}
