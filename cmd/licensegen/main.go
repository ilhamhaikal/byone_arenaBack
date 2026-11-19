// Command licensegen adalah tool VENDOR untuk membuat pasangan kunci dan
// menerbitkan lisensi offline untuk server klien.
//
// Tool ini tidak ikut dikirim ke klien. Private key hanya berada di komputer
// vendor (env LICENSE_PRIVATE_KEY atau file yang ditentukan).
//
// Contoh:
//
//	go run ./cmd/licensegen keygen -out-dir ./license-keys
//	go run ./cmd/licensegen issue -customer "Warnet Budi" -plan GROWTH \
//	    -units 10 -days 365 -instance <UUID-server-klien> -out budi.lic
//	go run ./cmd/licensegen inspect -license budi.lic -public ./license-keys/public.key
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"byone-arena/internal/licensegen"
	"byone-arena/pkg/licensekey"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "keygen":
		cmdKeygen(os.Args[2:])
	case "issue":
		cmdIssue(os.Args[2:])
	case "inspect":
		cmdInspect(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "perintah tidak dikenal: %s\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`licensegen — penerbit lisensi offline Byone Arena

Perintah:
  keygen    Membuat pasangan kunci (public + private)
  issue     Menerbitkan file lisensi (.lic)
  inspect   Memeriksa isi file lisensi

Contoh keygen:
  licensegen keygen -out-dir ./license-keys

Contoh issue:
  licensegen issue \
    -customer "Warnet Budi" -plan GROWTH \
    -units 10 -days 365 \
    -instance 11111111-2222-3333-4444-555555555555 \
    -out budi.lic

Contoh upgrade (tambah unit, expiry tetap):
  licensegen issue -customer "Warnet Budi" -plan GROWTH \
    -units 15 -expires 2027-11-17 \
    -instance 11111111-2222-3333-4444-555555555555 \
    -parent BYN-20261117-A1B2 -out budi-upgrade.lic

Catatan:
  -units 0 berarti tanpa batas unit (enterprise).
  Private key dibaca dari -key, env LICENSE_PRIVATE_KEY, atau
  ./license-keys/private.key.
`)
}

func cmdKeygen(args []string) {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	outDir := fs.String("out-dir", "./license-keys", "folder penyimpanan kunci")
	_ = fs.Parse(args)

	if err := os.MkdirAll(*outDir, 0o700); err != nil {
		fatal("gagal membuat folder: %v", err)
	}

	pub, priv, err := licensegen.GenerateKeypair()
	if err != nil {
		fatal("%v", err)
	}

	privPath := filepath.Join(*outDir, "private.key")
	pubPath := filepath.Join(*outDir, "public.key")

	if err := os.WriteFile(privPath, []byte(priv+"\n"), 0o600); err != nil {
		fatal("gagal menulis private key: %v", err)
	}
	if err := os.WriteFile(pubPath, []byte(pub+"\n"), 0o644); err != nil {
		fatal("gagal menulis public key: %v", err)
	}

	fmt.Println("✅ Pasangan kunci dibuat")
	fmt.Printf("   Private key : %s  (RAHASIA — jangan dibagikan)\n", privPath)
	fmt.Printf("   Public key  : %s\n\n", pubPath)
	fmt.Println("Tambahkan baris berikut ke .env server:")
	fmt.Println("LICENSE_PUBLIC_KEY=" + pub)
}

func cmdIssue(args []string) {
	fs := flag.NewFlagSet("issue", flag.ExitOnError)
	keyPath := fs.String("key", "", "file private key (default: LICENSE_PRIVATE_KEY / ./license-keys/private.key)")
	customer := fs.String("customer", "", "nama klien")
	plan := fs.String("plan", "", "nama paket (mis. STARTER/GROWTH/PRO/ENTERPRISE)")
	units := fs.Int("units", 0, "maksimal unit TV (0 = unlimited)")
	days := fs.Int("days", 365, "masa berlaku dalam hari")
	expires := fs.String("expires", "", "tanggal kedaluwarsa YYYY-MM-DD (menimpa -days)")
	instance := fs.String("instance", "", "ID server klien (UUID)")
	licenseID := fs.String("license-id", "", "ID lisensi (opsional)")
	parent := fs.String("parent", "", "ID lisensi sebelumnya (opsional, untuk upgrade)")
	out := fs.String("out", "license.lic", "file output lisensi")
	_ = fs.Parse(args)

	if strings.TrimSpace(*customer) == "" {
		fatal("-customer wajib diisi")
	}
	if strings.TrimSpace(*plan) == "" {
		fatal("-plan wajib diisi")
	}
	if strings.TrimSpace(*instance) == "" {
		fatal("-instance wajib diisi (ID server klien, lihat halaman Lisensi)")
	}
	if *units < 0 {
		fatal("-units tidak boleh negatif")
	}

	privB64 := readPrivateKey(*keyPath)

	exp := parseExpiry(*expires, *days)
	id := strings.TrimSpace(*licenseID)
	if id == "" {
		id = newLicenseID()
	}

	payload := licensekey.Payload{
		Version:         1,
		LicenseID:       id,
		Customer:        strings.TrimSpace(*customer),
		Plan:            strings.TrimSpace(*plan),
		MaxUnits:        *units,
		IssuedAt:        time.Now(),
		ExpiresAt:       exp,
		InstanceID:      strings.TrimSpace(*instance),
		ParentLicenseID: strings.TrimSpace(*parent),
	}

	text, err := licensegen.Issue(payload, privB64)
	if err != nil {
		fatal("gagal menerbitkan lisensi: %v", err)
	}
	if err := os.WriteFile(*out, []byte(text+"\n"), 0o644); err != nil {
		fatal("gagal menulis file lisensi: %v", err)
	}

	unitLabel := fmt.Sprintf("%d unit", payload.MaxUnits)
	if payload.MaxUnits == 0 {
		unitLabel = "unlimited"
	}
	fmt.Println("✅ Lisensi diterbitkan")
	fmt.Printf("   ID lisensi : %s\n", payload.LicenseID)
	fmt.Printf("   Klien      : %s\n", payload.Customer)
	fmt.Printf("   Paket      : %s (%s)\n", payload.Plan, unitLabel)
	fmt.Printf("   Berlaku    : %s s/d %s\n", payload.IssuedAt.Format("02-01-2006"),
		payload.ExpiresAt.Format("02-01-2006"))
	fmt.Printf("   Server     : %s\n", payload.InstanceID)
	fmt.Printf("   File       : %s\n", *out)
	if payload.ParentLicenseID != "" {
		fmt.Printf("   Upgrade dari: %s\n", payload.ParentLicenseID)
	}
}

func cmdInspect(args []string) {
	fs := flag.NewFlagSet("inspect", flag.ExitOnError)
	licPath := fs.String("license", "license.lic", "file lisensi")
	pubFlag := fs.String("public", "", "file public key atau base64 public key")
	_ = fs.Parse(args)

	raw, err := os.ReadFile(*licPath)
	if err != nil {
		fatal("gagal membaca lisensi: %v", err)
	}
	pubB64 := readPublicKey(*pubFlag)

	pub, err := licensekey.ParsePublicKey(pubB64)
	if err != nil {
		fatal("%v", err)
	}
	p, err := licensekey.Verify(string(raw), pub)
	if err != nil {
		fatal("lisensi TIDAK VALID: %v", err)
	}

	pretty, _ := json.MarshalIndent(p, "", "  ")
	fmt.Println("✅ Lisensi valid. Isi:")
	fmt.Println(string(pretty))
}

// ── util ────────────────────────────────────────────────────────────────────

func parseExpiry(explicit string, days int) time.Time {
	if strings.TrimSpace(explicit) != "" {
		t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(explicit), time.Local)
		if err != nil {
			fatal("format -expires harus YYYY-MM-DD: %v", err)
		}
		return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.Local)
	}
	if days <= 0 {
		fatal("-days harus lebih dari 0")
	}
	d := time.Now().AddDate(0, 0, days)
	return time.Date(d.Year(), d.Month(), d.Day(), 23, 59, 59, 0, time.Local)
}

func newLicenseID() string {
	b := make([]byte, 2)
	if _, err := rand.Read(b); err != nil {
		return "BYN-" + time.Now().Format("20060102")
	}
	return "BYN-" + time.Now().Format("20060102") + "-" +
		strings.ToUpper(hex.EncodeToString(b))
}

func readPrivateKey(path string) string {
	if strings.TrimSpace(path) != "" {
		return readTrimmedFile(path, "private key")
	}
	if env := strings.TrimSpace(os.Getenv("LICENSE_PRIVATE_KEY")); env != "" {
		// Boleh berupa base64 langsung, atau path file.
		if st, err := os.Stat(env); err == nil && !st.IsDir() {
			return readTrimmedFile(env, "private key")
		}
		return env
	}
	return readTrimmedFile(filepath.Join("license-keys", "private.key"), "private key")
}

func readPublicKey(flagVal string) string {
	if strings.TrimSpace(flagVal) != "" {
		if st, err := os.Stat(flagVal); err == nil && !st.IsDir() {
			return readTrimmedFile(flagVal, "public key")
		}
		return strings.TrimSpace(flagVal)
	}
	if env := strings.TrimSpace(os.Getenv("LICENSE_PUBLIC_KEY")); env != "" {
		if st, err := os.Stat(env); err == nil && !st.IsDir() {
			return readTrimmedFile(env, "public key")
		}
		return env
	}
	return readTrimmedFile(filepath.Join("license-keys", "public.key"), "public key")
}

func readTrimmedFile(path, label string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		fatal("gagal membaca %s (%s): %v", label, path, err)
	}
	return strings.TrimSpace(string(b))
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "❌ "+format+"\n", args...)
	os.Exit(1)
}
