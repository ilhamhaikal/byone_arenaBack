package deviceauth

import "testing"

func TestHashSecretAndCompare(t *testing.T) {
	secret := "contoh-secret-perangkat"
	h := HashSecret(secret)
	if !IsValidHash(h) {
		t.Fatalf("hash tidak valid: %q", h)
	}
	if !EqualHash(h, HashSecret(secret)) {
		t.Fatal("hash yang sama harus cocok")
	}
	if EqualHash(h, HashSecret("secret-lain")) {
		t.Fatal("hash berbeda tidak boleh cocok")
	}
	if EqualHash(h, "") {
		t.Fatal("hash kosong tidak boleh cocok")
	}
}

func TestIsValidHash(t *testing.T) {
	if IsValidHash("bukan-hash") {
		t.Fatal("string bukan hash tidak boleh valid")
	}
	if IsValidHash("") {
		t.Fatal("hash kosong tidak boleh valid")
	}
}
