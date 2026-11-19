# Windows: Mengurangi Blokir Antivirus & Code Signing

Dokumen ini untuk **vendor/developer**. Tujuan: meminimalkan kasus
`byone-server.exe` / installer diblokir Windows Defender atau SmartScreen.

> Penting: **code signing** di sini adalah *Authenticode* (identitas penerbit
> untuk Windows), **bukan** signature lisensi Ed25519. Keduanya berbeda.

## 1. Kenapa sering diblokir

- EXE tidak ditandatangani → penerbit "Unknown".
- Belum punya reputasi (baru, sedikit pengguna, dikirim lewat ZIP/WhatsApp).
- Heuristik antivirus sering salah tuduh binary Go (statically linked).
- File hasil unduhan membawa *Mark-of-the-Web*.
- Server membuka port → memicu prompt firewall (bukan antivirus).

## 2. Mitigasi gratis (sudah disiapkan)

- **Icon + version info** ditempel ke EXE (CompanyName, ProductName,
  FileDescription, FileVersion, LegalCopyright, OriginalFilename).
- **Manifest** `asInvoker` + kompatibilitas Windows 10/11 + long path aware.
- **Installer NSIS** (bukan ZIP) yang memasang ke `Program Files`,
  menambah **firewall rule** port 8080, dan bisa **auto-start** via
  Scheduled Task.
- **Checksum SHA-256** disertakan (`dist/windows/SHA256SUMS.txt`).
- Skrip cmd (`start-server.cmd`, `run-migrate.cmd`) memastikan working
  directory benar.

### Build paket Windows

```bash
# sekali saja
go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
sudo apt-get install -y nsis

# setiap rilis
VERSION=1.0.1 ./scripts/build_windows.sh
```

Hasil:
```
dist/windows/byone-arena-setup-1.0.1.exe
dist/windows/payload/byone-server.exe
dist/windows/payload/migrate.exe
dist/windows/SHA256SUMS.txt
```

## 3. Lapor false positive (gratis, wajib dicoba dulu)

1. Buka <https://www.microsoft.com/en-us/wdsi/filesubmission>
2. Pilih **Home customer** / **Software developer**.
3. Unggah installer atau EXE yang dituduh.
4. Isi deskripsi: "Legitimate Point-of-Sale server for game rental, built with Go. False positive."
5. Sertakan SHA-256 dari `SHA256SUMS.txt`.

Biasanya diproses beberapa jam–hari. Jika file berubah (versi baru),
mungkin perlu lapor lagi.

## 4. Code signing (opsional, pengurang terbesar)

| Opsi | Biaya (kisaran) | Efek | Catatan |
|---|---|---|---|
| OV certificate | ±Rp 3–6 jt/tahun | Nama penerbit muncul; reputasi SmartScreen bertahap | Perlu dokumen usaha |
| EV certificate | ±Rp 6–12 jt/tahun | Kepercayaan SmartScreen instan | Biasanya hardware token |
| Azure Trusted Signing | Langganan bulanan | Mirip OV/EV, tanpa token fisik | Ada syarat kelayakan |

Code signing **tidak menjamin 0 peringatan**, tetapi menghilangkan
"Unknown publisher" dan menurunkan drastis flag AV.

### Menandatangani dari Windows

```bat
signtool sign /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 /a ^
  "dist\windows\payload\byone-server.exe"
signtool sign /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 /a ^
  "dist\windows\payload\migrate.exe"
signtool sign /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 /a ^
  "dist\windows\byone-arena-setup-1.0.1.exe"
```

### Menandatangani dari Linux

```bash
sudo apt-get install -y osslsigncode
osslsigncode sign -pkcs12 cert.pfx -pass "$PFX_PASS" \
  -n "Byone Arena Server" -i "https://example.com" \
  -ts http://timestamp.digicert.com \
  -in dist/windows/payload/byone-server.exe \
  -out dist/windows/payload/byone-server-signed.exe
```

Lakukan untuk `byone-server.exe`, `migrate.exe`, dan installer, lalu
bangun ulang installer dengan file yang sudah ditandatangani.

## 5. Batasan yang jujur

| Kondisi | Tanpa sertifikat | Dengan sertifikat |
|---|---|---|
| Defender salah tuduh | Bisa dilaporkan, menunggu | Lebih jarang |
| SmartScreen "Unknown publisher" | Muncul untuk file baru | Berkurang/hilang |
| Windows 11 Smart App Control | **Bisa blokir total** | Diperlukan signing |
| Banyak klien & sering update | Reputasi tidak terbangun | Lebih stabil |

## 6. Checklist setiap rilis

- [ ] Naikkan `VERSION` (mis. `VERSION=1.0.1 ./scripts/build_windows.sh`)
- [ ] Uji installer di Windows 10/11 bersih
- [ ] Cek di <https://www.virustotal.com>
- [ ] Jika ada flag: lapor false positive + simpan nomor tiket
- [ ] (Opsional) tanda tangani EXE + installer, lalu ulangi build
- [ ] Simpan `SHA256SUMS.txt` bersama rilis
