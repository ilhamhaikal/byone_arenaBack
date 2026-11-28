package license

import "testing"

func TestExtractLicenseKey(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "file teks vendor",
			in: "ByoneBilling — Lisensi\nID Lisensi : LIC-2026-G6KVEENIZN\n" +
				"Kunci      : BYA-5C4G-Y4G1-5Z6K-V0B0-F8Y3\nBerlaku    : 28 Nov 2026\n",
			want: "BYA-5C4G-Y4G1-5Z6K-V0B0-F8Y3",
		},
		{
			name: "kunci dengan spasi",
			in:   "bya 5c4g y4g1 5z6k v0b0 f8y3",
			want: "BYA5C4GY4G15Z6KV0B0F8Y3",
		},
		{
			name: "kanonik tanpa prefix",
			in:   "5C4G-Y4G1-5Z6K-V0B0-F8Y3",
			want: "5C4GY4G15Z6KV0B0F8Y3",
		},
		{name: "bukan kunci", in: "eyJ2IjoxLCJsaWNlbnNlSWQiOiJMSUMifQ.sig", want: ""},
		{name: "kosong", in: "   ", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractLicenseKey(tc.in); got != tc.want {
				t.Fatalf("extractLicenseKey(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
