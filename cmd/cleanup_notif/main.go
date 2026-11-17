//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"log"

	"byone-arena/pkg/config"
	"byone-arena/pkg/database"
	"byone-arena/pkg/spname"
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
	var r struct {
		Updated int64 `gorm:"column:updated"`
	}
	tx := db.Raw("SELECT * FROM "+spname.Ident("DeactivateTvNotificationByTitle")+"(?::text)",
		"Sesi Diperpanjang").Scan(&r)
	if tx.Error != nil {
		log.Fatal("Cleanup error:", tx.Error)
	}
	fmt.Printf("✅ Cleaned up %d stale 'Sesi Diperpanjang' notifications\n", r.Updated)

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
