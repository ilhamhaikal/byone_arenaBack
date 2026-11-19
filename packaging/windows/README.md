# Packaging Windows

Folder ini berisi bahan untuk membangun paket Windows Byone Arena.

| File | Fungsi |
|---|---|
| `app.ico` | Icon aplikasi (multi-ukuran) |
| `app.manifest` | Manifest Windows (template; `__VERSION__` diisi otomatis) |
| `versioninfo-server.json` | Version info `byone-server.exe` |
| `versioninfo-migrate.json` | Version info `migrate.exe` |
| `installer.nsi` | Skrip installer NSIS |

## Cara build

Jangan memanggil `makensis`/`goversioninfo` manual. Gunakan:

```bash
VERSION=1.0.0 ./scripts/build_windows.sh
```

Skrip akan:
1. Mengisi versi ke version info & manifest.
2. Menghasilkan `resource_windows_amd64.syso` untuk `cmd/server` dan `cmd/migrate`.
3. Cross-compile `GOOS=windows`.
4. Menyiapkan payload (migrations, `.env.example`, docs, skrip cmd).
5. Membuat installer NSIS + `SHA256SUMS.txt`.

Hasil di `dist/windows/`.

## Catatan

- `app.manifest` sengaja memakai `__VERSION__` (bukan angka) karena versi
  diisi saat build.
- `.syso` yang sudah dihasilkan ikut disimpan agar `go build` biasa (tanpa
  skrip) tetap membawa icon & version info.
- Installer **belum ditandatangani**. Lihat `docs/WINDOWS_CODE_SIGNING.md`.
