// +build ignore

package main

import (
	"fmt"
	"log"

	"byone-arena/pkg/config"
	"byone-arena/pkg/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Config load error:", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatal("DB connect error:", err)
	}

	// Cleanup stale "Sesi Diperpanjang" notifications
	result := db.Exec(`UPDATE tv_notifications SET is_active = false, updated_at = NOW() WHERE title = 'Sesi Diperpanjang' AND is_active = true`)
	if result.Error != nil {
		log.Fatal("Cleanup error:", result.Error)
	}
	fmt.Printf("✅ Cleaned up %d stale 'Sesi Diperpanjang' notifications\n", result.RowsAffected)

	// Show remaining active notifications
	type Notif struct {
		ID      string `gorm:"column:id"`
		Title   string `gorm:"column:title"`
		Message string `gorm:"column:message"`
		Active  bool   `gorm:"column:is_active"`
	}
	var notifs []Notif
	db.Where("is_active = true").Order("created_at DESC").Limit(10).Find(&notifs)
	fmt.Println("\nActive notifications remaining:")
	for _, n := range notifs {
		fmt.Printf("  [%s] %s: %s\n", n.Title, n.Active, n.Message[:min(80, len(n.Message))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
