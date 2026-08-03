# Byone Arena TV Launcher & Smart Screen Saver

## 1. Tujuan

Byone Arena dirancang sebagai aplikasi Android TV yang berfungsi sebagai **Launcher**, **Screen Saver**, dan **Client Rental**, tanpa menghilangkan fungsi utama televisi.

Aplikasi tidak mengubah TV menjadi perangkat kiosk yang terkunci. Sebaliknya, aplikasi menjadi pintu masuk (entry point) saat TV dinyalakan dan akan kembali tampil secara otomatis ketika sesi penyewaan berakhir atau ketika sistem menginstruksikannya.

---

# 2. Latar Belakang

Pada umumnya Android TV digunakan untuk:

* Menonton YouTube
* Menonton Netflix
* Menggunakan aplikasi streaming lainnya
* Memutar HDMI
* Menonton TV Digital

Sistem Byone Arena tidak bertujuan membatasi aktivitas tersebut.

Sebaliknya, sistem bertugas mengelola akses penyewa terhadap perangkat TV sehingga penggunaan tetap terkontrol sesuai durasi penyewaan.

---

# 3. Tujuan Sistem

Sistem harus mampu:

* Menampilkan aplikasi Byone Arena setiap TV dinyalakan.
* Menampilkan informasi status rental.
* Menampilkan Screen Saver ketika TV sedang tidak digunakan.
* Mengizinkan penyewa menggunakan seluruh fitur Android TV.
* Mengembalikan tampilan ke Byone Arena secara otomatis ketika sesi selesai atau aplikasi admin mengirim sinyal off atau screen sever.

---

# 4. Konsep Sistem

```
Power ON TV
      │
      ▼
Android Boot
      │
      ▼
Byone Arena Launcher
      │
      ▼
Screen Saver
      │
      ▼
User menekan LIVE
      │
      ▼
Launcher Android TV
      │
      ├── Netflix
      ├── YouTube
      ├── HDMI
      ├── TV Digital
      └── Aplikasi lain
      │
      ▼
Rental Berakhir
      │
      ▼
Byone Arena tampil kembali
```

---

# 5. Mekanisme Boot

## Kondisi

TV dalam keadaan mati (tidak mendapat listrik).

Operator menghidupkan TV menggunakan remote.

## Sistem

Android melakukan proses boot.

Setelah boot selesai:

1. Sistem memanggil Launcher Byone Arena.
2. Aplikasi tampil otomatis.
3. Tidak diperlukan interaksi pengguna.

Output yang diharapkan:

```
TV Menyala

↓

Screen Saver dari sistem byone arena
```

---

# 6. Screen Saver

Screen Saver merupakan tampilan default aplikasi.

Contoh informasi yang ditampilkan:

* Logo Byone Arena
* Jam
* Status perangkat
* Animasi

Contoh:

```
==============================

        BYONE ARENA

      Selamat Datang

      TV Siap Digunakan

      [ LIVE ]

==============================
```

---

# 7. Tombol LIVE

Ketika tombol LIVE ditekan:

Byone Arena tidak menutup aplikasi.

Aplikasi hanya menyerahkan kontrol kepada Android Launcher.

Alur:

```
Klik LIVE

↓

Launcher Android

↓

User bebas menggunakan TV
```

---

# 8. Aktivitas Penyewa

Selama status LIVE aktif, penyewa bebas menggunakan:

* Netflix
* YouTube
* Disney+
* Spotify
* Browser
* HDMI
* TV Digital
* Aplikasi Android lainnya

Remote tetap bekerja normal.

Tidak ada pembatasan tombol Home, Back, Direction maupun Volume.

---

# 9. Status Rental

Server memiliki beberapa status.

## Idle

```
TV belum disewa.

↓

Byone Arena tampil.
```

---

## Active

```
TV sedang disewa.

↓

User bebas menggunakan TV.
```

---

## Expired

```
Waktu habis.

↓

Byone Arena tampil kembali.
```

---

## Maintenance

```
TV tidak dapat digunakan.

↓

Menampilkan halaman Maintenance.
```

---

# 10. Mekanisme Pengembalian

Byone Arena harus mampu kembali tampil secara otomatis apabila:

* waktu rental habis;
* admin menghentikan sesi;
* server mengirim perintah penghentian;
* perangkat kembali ke mode idle.

Contoh:

```
Netflix

↓

Waktu habis

↓

Server

↓

WebSocket

↓

Byone Arena tampil
```

---

# 11. Komunikasi Backend

Aplikasi Android TV selalu terhubung menggunakan WebSocket.

Contoh event:

```
SESSION_STARTED

SESSION_ENDED

SESSION_CANCELLED

SESSION_EXTENDED

PAYMENT_CREATED

PAYMENT_CONFIRMED

PAYMENT_REFUNDED

CONSOLE_UPDATED

PING

TV_WAKE

TV_SLEEP

TV_SCREENSAVER

TV_NOTIFICATION
```

---

# 12. Peran Aplikasi

Aplikasi Android TV hanya memiliki satu mode.

```
CLIENT
```

Tidak ada pilihan:

* Admin
* Operator
* Client

Karena seluruh perangkat Android TV secara otomatis dianggap sebagai Client.

Sedangkan:

* Desktop
* Web
* Android Phone
* Tablet

menggunakan mekanisme login biasa.

---

# 13. Identifikasi Perangkat

Pada saat pertama kali dijalankan, aplikasi melakukan identifikasi perangkat.

Jika perangkat merupakan Android TV:

```
Mode = CLIENT
```

Jika bukan Android TV:

```
Tampilkan Login
```

Dengan demikian pengguna TV tidak pernah melihat halaman pemilihan mode.

---

# 14. Keuntungan Arsitektur

* TV langsung membuka Byone Arena saat dinyalakan.
* Tidak perlu membuka aplikasi secara manual.
* Penyewa tetap bebas menggunakan Android TV.
* Remote tetap berfungsi normal.
* Netflix, YouTube, HDMI dan TV Digital tetap dapat digunakan.
* Tidak menggunakan Kiosk Mode sehingga pengalaman pengguna tetap alami.
* Kontrol penuh berada di backend.
* Pengelolaan status perangkat menjadi lebih mudah.
* Mendukung pembaruan konten secara real-time melalui WebSocket untuk client.

---

# 15. Kesimpulan

Byone Arena bukan sekadar aplikasi Android TV, melainkan sebuah sistem manajemen perangkat yang bertindak sebagai Launcher, Screen Saver, dan Client Rental.

Pendekatan ini memungkinkan TV tetap berfungsi seperti televisi pada umumnya, sementara seluruh proses penyewaan, pengawasan, dan pengendalian dilakukan secara otomatis melalui backend. Dengan desain ini, pengalaman pengguna tetap nyaman, fungsi televisi tidak terganggu, dan operator memiliki kendali penuh terhadap status setiap perangkat. dan seharusnya koneksi websoket dan http itu
dipisah agar lebih clean architectur baik itu di sisi backend dan frontend,
