package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"
)

// Probe adalah teks yang dikirim aplikasi TV lewat UDP broadcast. Server hanya
// membalas datagram yang mengandung teks ini, sehingga port tidak "berisik".
const Probe = "BYONE_DISCOVER"

type reply struct {
	Service string `json:"service"`
	Name    string `json:"name"`
	API     string `json:"api"`
	Version string `json:"version"`
}

// Start menjalankan responder discovery di port UDP tertentu.
//
// Desain: TV melakukan broadcast probe -> server membalas secara unicast ke
// alamat pengirim. Dengan cara ini server tidak perlu izin SO_BROADCAST dan
// alamat IP yang dikirim selalu IP interface yang benar-benar dipakai untuk
// menjangkau TV (bukan tebakan dari daftar interface).
func Start(ctx context.Context, port int, apiPort, appName, version string) error {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: port})
	if err != nil {
		return err
	}

	go func() {
		defer conn.Close()
		buf := make([]byte, 512)

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, from, err := conn.ReadFromUDP(buf)
			if err != nil {
				continue // timeout / sementara; cek ctx di iterasi berikutnya
			}
			if !strings.Contains(strings.ToUpper(string(buf[:n])), Probe) {
				continue
			}

			// Cari IP lokal yang dipakai untuk menjangkau pengirim.
			d, derr := net.DialUDP("udp4", nil, from)
			if derr != nil {
				continue
			}
			localIP := d.LocalAddr().(*net.UDPAddr).IP.String()
			_ = d.Close()

			payload, merr := json.Marshal(reply{
				Service: "byone-arena",
				Name:    appName,
				API:     fmt.Sprintf("http://%s:%s/api/v1", localIP, apiPort),
				Version: version,
			})
			if merr != nil {
				continue
			}
			_, _ = conn.WriteToUDP(payload, from)
		}
	}()

	return nil
}