// +build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	dir := "migrations"
	entries, _ := os.ReadDir(dir)

	var upFiles []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".up.sql") {
			upFiles = append(upFiles, name)
		}
	}
	sort.Strings(upFiles)

	// Gabungkan semua .up.sql jadi satu file consolidated
	var allSQL strings.Builder
	allSQL.WriteString("-- ============================================\n")
	allSQL.WriteString("-- CONSOLIDATED MIGRATION (all 34 migrations merged)\n")
	allSQL.WriteString("-- Generated: 2026-07-23\n")
	allSQL.WriteString("-- ============================================\n\n")

	for _, f := range upFiles {
		content, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			fmt.Printf("SKIP %s: %v\n", f, err)
			continue
		}
		allSQL.WriteString(fmt.Sprintf("\n-- ===== FROM: %s =====\n\n", f))
		allSQL.WriteString(string(content))
		allSQL.WriteString("\n")
	}

	outFile := filepath.Join(dir, "000_ALL_CONSOLIDATED.up.sql")
	os.WriteFile(outFile, []byte(allSQL.String()), 0644)
	fmt.Printf("✅ Written: %s (%d bytes)\n", outFile, allSQL.Len())
	fmt.Printf("   Total files merged: %d\n", len(upFiles))
}
