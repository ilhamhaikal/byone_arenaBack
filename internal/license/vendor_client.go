package license

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrOnlineDisabled dikembalikan bila LICENSE_SERVER_URL belum diisi.
var ErrOnlineDisabled = errors.New("license server vendor belum dikonfigurasi")

// VendorClient adalah klien HTTP ke license server vendor. Hanya server
// billing yang menghubungi endpoint ini; TV/perangkat LAN tidak.
type VendorClient struct {
	baseURL    string
	appVersion string
	http       *http.Client
}

// NewVendorClient membuat klien vendor. baseURL harus tanpa trailing slash.
func NewVendorClient(baseURL, appVersion string, timeout time.Duration) *VendorClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	// Jangan biarkan server vendor mengalihkan request ke host lain.
	if parsed, err := url.Parse(baseURL); err == nil && parsed.Host != "" {
		client.CheckRedirect = func(req *http.Request, _ []*http.Request) error {
			if !strings.EqualFold(req.URL.Host, parsed.Host) {
				return errors.New("redirect ke host lain ditolak")
			}
			return nil
		}
	}
	return &VendorClient{baseURL: baseURL, appVersion: appVersion, http: client}
}

// Enabled true bila URL vendor tersedia.
func (c *VendorClient) Enabled() bool { return c != nil && c.baseURL != "" }

// VendorAPI adalah kontrak minimum integrasi license server vendor.
type VendorAPI interface {
	Enabled() bool
	CreateActivationRequest(ctx context.Context, instanceID, machineLabel, osInfo string) (*VendorActivationRequest, error)
	PollActivationRequest(ctx context.Context, code, secret string) (*VendorActivationPoll, error)
	Activate(ctx context.Context, licenseKey, instanceID, machineLabel, osInfo string) (*VendorActivate, error)
	Heartbeat(ctx context.Context, licenseToken, instanceID string) (*VendorHeartbeat, error)
}

// VendorActivationRequest adalah respons pembuatan kode aktivasi.
type VendorActivationRequest struct {
	Code             string    `json:"code"`
	Secret           string    `json:"secret"`
	ExpiresAt        time.Time `json:"expiresAt"`
	PollAfterSeconds int       `json:"pollAfterSeconds"`
}

// VendorActivationPoll adalah respons polling aktivasi.
type VendorActivationPoll struct {
	Status    string     `json:"status"`
	Valid     bool       `json:"valid"`
	Code      string     `json:"code,omitempty"`
	Message   string     `json:"message,omitempty"`
	License   string     `json:"license,omitempty"`
	LicenseID string     `json:"licenseId,omitempty"`
	Customer  string     `json:"customer,omitempty"`
	Plan      string     `json:"plan,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

// VendorActivate adalah respons aktivasi manual memakai kunci lisensi.
type VendorActivate struct {
	Valid      bool      `json:"-"`
	License    string    `json:"license"`
	LicenseID  string    `json:"licenseId"`
	Customer   string    `json:"customer"`
	Plan       string    `json:"plan"`
	PlanName   string    `json:"planName"`
	MaxUnits   int       `json:"maxUnits"`
	Features   []string  `json:"features"`
	IssuedAt   time.Time `json:"issuedAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
	GraceDays  int       `json:"graceDays"`
	Activation string    `json:"activationId"`
	ServerTime time.Time `json:"serverTime"`
}

// VendorHeartbeat adalah respons heartbeat/revocation check.
type VendorHeartbeat struct {
	Valid       bool       `json:"valid"`
	Code        string     `json:"code,omitempty"`
	Message     string     `json:"message,omitempty"`
	License     string     `json:"license,omitempty"`
	State       string     `json:"state,omitempty"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
	GraceEndsAt *time.Time `json:"graceEndsAt,omitempty"`
	ServerTime  time.Time  `json:"serverTime"`
}

type vendorEnvelope struct {
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// VendorAPIError menjelaskan penolakan dari license server vendor.
type VendorAPIError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *VendorAPIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("license server menolak request (HTTP %d)", e.StatusCode)
}

func (c *VendorClient) post(ctx context.Context, path string, body, out any) error {
	if !c.Enabled() {
		return ErrOnlineDisabled
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ByoneBilling/"+c.appVersion)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("tidak dapat menghubungi license server: %w", err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, 1<<20)
	payload, err := io.ReadAll(limited)
	if err != nil {
		return fmt.Errorf("gagal membaca respons license server: %w", err)
	}
	var envelope vendorEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return fmt.Errorf("respons license server tidak valid (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &VendorAPIError{StatusCode: resp.StatusCode, Code: "http_error"}
		if envelope.Error != nil {
			apiErr.Code = envelope.Error.Code
			apiErr.Message = envelope.Error.Message
		}
		return apiErr
	}
	if out != nil {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return fmt.Errorf("data license server tidak valid: %w", err)
		}
	}
	return nil
}

// CreateActivationRequest meminta kode aktivasi sekali pakai ke vendor.
func (c *VendorClient) CreateActivationRequest(ctx context.Context, instanceID, machineLabel, osInfo string) (*VendorActivationRequest, error) {
	var out VendorActivationRequest
	err := c.post(ctx, "/api/v1/activations/requests", map[string]any{
		"instanceId":   instanceID,
		"machineLabel": machineLabel,
		"appVersion":   c.appVersion,
		"osInfo":       osInfo,
	}, &out)
	if err != nil {
		return nil, err
	}
	if out.Code == "" || out.Secret == "" {
		return nil, errors.New("license server tidak mengembalikan kode aktivasi")
	}
	if out.PollAfterSeconds < 2 {
		out.PollAfterSeconds = 5
	}
	return &out, nil
}

// PollActivationRequest memeriksa apakah vendor sudah menerbitkan lisensi.
func (c *VendorClient) PollActivationRequest(ctx context.Context, code, secret string) (*VendorActivationPoll, error) {
	var out VendorActivationPoll
	err := c.post(ctx, "/api/v1/activations/requests/"+url.PathEscape(code)+"/poll", map[string]any{
		"code": code, "secret": secret,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Activate menukar kunci lisensi dengan token bertanda tangan terbaru.
// Endpoint ini mewajibkan server billing terhubung ke internet.
func (c *VendorClient) Activate(ctx context.Context, licenseKey, instanceID, machineLabel, osInfo string) (*VendorActivate, error) {
	var out VendorActivate
	err := c.post(ctx, "/api/v1/activations/", map[string]any{
		"licenseKey":   licenseKey,
		"instanceId":   instanceID,
		"machineLabel": machineLabel,
		"appVersion":   c.appVersion,
		"osInfo":       osInfo,
	}, &out)
	if err != nil {
		return nil, err
	}
	if out.License == "" {
		return nil, errors.New("license server tidak mengembalikan token lisensi")
	}
	out.Valid = true
	return &out, nil
}

// Heartbeat mengirim token lisensi untuk update last-seen dan revocation.
func (c *VendorClient) Heartbeat(ctx context.Context, licenseToken, instanceID string) (*VendorHeartbeat, error) {
	var out VendorHeartbeat
	err := c.post(ctx, "/api/v1/activations/heartbeat", map[string]any{
		"license":    licenseToken,
		"instanceId": instanceID,
		"appVersion": c.appVersion,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
