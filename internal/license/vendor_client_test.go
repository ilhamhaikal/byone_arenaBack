package license

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestVendorClientFlow(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/activations/requests", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != "ByoneBilling/9.9.9" {
			t.Errorf("user agent = %q", got)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["instanceId"] != "instance-1" {
			t.Errorf("instanceId = %v", body["instanceId"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"code": "REQ-1", "secret": "s3cret", "expiresAt": time.Now().Add(time.Hour),
			"pollAfterSeconds": 5,
		}})
	})
	mux.HandleFunc("/api/v1/activations/requests/REQ-1/poll", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"status": "pending", "valid": false, "message": "menunggu",
		}})
	})
	mux.HandleFunc("/api/v1/activations/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"valid": true, "state": "active", "license": "token",
		}})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewVendorClient(server.URL, "9.9.9", 5*time.Second)
	ctx := context.Background()
	req, err := client.CreateActivationRequest(ctx, "instance-1", "PC Kasir", "Windows")
	if err != nil || req.Code != "REQ-1" || req.Secret != "s3cret" {
		t.Fatalf("create = %+v, err=%v", req, err)
	}
	poll, err := client.PollActivationRequest(ctx, req.Code, req.Secret)
	if err != nil || poll.Valid || poll.Status != "pending" {
		t.Fatalf("poll = %+v, err=%v", poll, err)
	}
	hb, err := client.Heartbeat(ctx, "token", "instance-1")
	if err != nil || !hb.Valid {
		t.Fatalf("heartbeat = %+v, err=%v", hb, err)
	}
}

func TestVendorClientRejectsErrorEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":"license_suspended","message":"ditangguhkan"}}`))
	}))
	defer server.Close()

	client := NewVendorClient(server.URL, "1.0.0", time.Second)
	_, err := client.Heartbeat(context.Background(), "token", "instance")
	if err == nil || !strings.Contains(err.Error(), "ditangguhkan") {
		t.Fatalf("expected vendor error, got %v", err)
	}
}

func TestSecretBox(t *testing.T) {
	box := newSecretBox("server-secret")
	sealed, err := box.seal("secret-value")
	if err != nil {
		t.Fatal(err)
	}
	if sealed == "secret-value" {
		t.Fatal("secret tidak terenkripsi")
	}
	plain, err := box.open(sealed)
	if err != nil || plain != "secret-value" {
		t.Fatalf("open = %q, err=%v", plain, err)
	}
	if _, err := box.open(sealed + "x"); err == nil {
		t.Fatal("tampered secret harus ditolak")
	}
}
