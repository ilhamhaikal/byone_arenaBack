package usecase

import "byone-arena/internal/domain/entity"

// CreateNotificationRequest payload untuk membuat notifikasi TV
type CreateNotificationRequest struct {
	Title        string                      `json:"title"        validate:"required,min=2,max=100" example:"Promo Spesial!"`
	Message      string                      `json:"message"      validate:"required,min=2,max=1000" example:"Diskon 20% jam 14:00-17:00!"`
	ImageURL     *string                     `json:"imageUrl"     validate:"omitempty,url" example:"https://example.com/promo.jpg"`
	Priority     entity.NotificationPriority `json:"priority"     validate:"omitempty,oneof=low normal high" example:"normal"`
	LoopEnabled  bool                        `json:"loopEnabled"  example:"false"`
	LoopInterval int                         `json:"loopInterval" validate:"omitempty,gte=5" example:"30"`
	TargetAll    bool                        `json:"targetAll"    example:"true"`
	// ConsoleIDs — daftar UUID konsol yang ditarget (jika targetAll=false)
	ConsoleIDs         []string `json:"consoleIds"         example:"[\"bd7edb34-...\",\"b49b20d9-...\"]"`
	// ActiveSessionsOnly — hanya kirim ke konsol dengan sesi aktif
	ActiveSessionsOnly bool     `json:"activeSessionsOnly"  example:"false"`
}
