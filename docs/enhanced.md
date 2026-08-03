# Byone Arena Client - Android TV Overlay Architecture

# Tujuan

Saya sedang mengembangkan sistem manajemen rental PlayStation bernama **Byone Arena**.

Saya ingin Anda menganalisa arsitektur aplikasi Client Android TV.

JANGAN langsung menulis kode.

Saya ingin analisa arsitektur terlebih dahulu.

---

# Gambaran Sistem

Project terdiri dari tiga komponen utama.

1. Flutter Desktop (Admin)
2. Golang Backend
3. Flutter Android TV (Client)

Semua komponen berada pada jaringan LAN yang sama.

---

# Arsitektur

```
                Flutter Desktop
                    (Admin)
                       │
                 REST API
                       │
               Golang Backend
                       │
        ┌──────────────┴──────────────┐
        │                             │
     REST API                     WebSocket
        │                             │
        └──────────────┬──────────────┘
                       │
                Android TV Client
```

Backend adalah sumber kebenaran (Source of Truth).

Semua perubahan status console berasal dari Backend.

---

# Komunikasi

Client menggunakan dua mekanisme komunikasi.

## REST API

REST digunakan untuk sinkronisasi data.

Misalnya:

- status console
- data sesi
- konfigurasi
- harga
- promo
- informasi screen saver

REST berjalan secara periodik.

Misalnya:

setiap 10 detik.

REST bukan media utama untuk event realtime.

---

## WebSocket

Client harus selalu terhubung ke WebSocket.

WebSocket digunakan untuk event realtime.

Misalnya:

- session dibuat
- session dimulai
- session diperpanjang
- session selesai
- maintenance aktif
- maintenance selesai
- promo baru
- countdown
- warning
- refresh UI

Saat menerima event WebSocket,

UI harus berubah saat itu juga.

Tanpa menunggu polling REST berikutnya.

Jika WebSocket terputus,

Client harus reconnect otomatis.

Polling REST tetap berjalan sebagai fallback.

---

# Fungsi Client

Ini bagian yang PALING PENTING.

Client BUKAN launcher.

Client BUKAN aplikasi game.

Client BUKAN media player.

Client BUKAN aplikasi utama TV.

Client hanya bertugas menampilkan informasi kepada pemain.

Client tidak boleh mengambil alih fungsi Android TV.

---

# State Client

Client mempunyai state berikut.

- loading
- idle
- active
- overtime
- maintenance

---

# State Loading

Saat aplikasi pertama dijalankan.

Yang ditampilkan:

- Logo
- Loading
- Menghubungkan Backend
- Membuka WebSocket

---

# State Idle

Saat console belum digunakan.

Yang ditampilkan:

- Screen Saver
- Jam
- Logo
- Promo
- Informasi Arena
- Particle

Screen Saver fullscreen.

---

# State Active

Ini adalah requirement paling penting.

Saat state ACTIVE,

berarti pemain sedang menggunakan console.

Misalnya:

- Bermain Game
- Menonton YouTube
- Netflix
- Emulator
- HDMI
- Launcher lain

Yang saya inginkan adalah:

TV tetap menampilkan aplikasi yang sedang digunakan.

Client tidak boleh membuat layar menjadi hitam.

Client tidak boleh membuat Scaffold fullscreen.

Client tidak boleh menggambar Container fullscreen.

Client tidak boleh menggambar widget kosong yang menutupi layar.

Client hanya boleh menampilkan informasi berikut.

- indikator LIVE
- warning
- countdown
- notifikasi
- sisa waktu

Apabila tidak ada informasi,

overlay hampir tidak terlihat.

Dengan kata lain,

Client adalah lapisan informasi,

BUKAN aplikasi utama.

---

# State Warning

Saat sisa waktu <=5 menit.

Munculkan popup kecil.

Misalnya:

------------------------------

Sisa waktu tinggal 5 menit

------------------------------

Popup muncul beberapa detik.

Lalu hilang otomatis.

---

# Countdown

Saat sisa <=10 detik.

Tampilkan countdown.

10

9

8

...

1

---

# State Overtime

Saat waktu habis.

Barulah Client mengambil alih layar.

Tampilkan fullscreen.

Contoh.

=========================

WAKTU HABIS

Hubungi Admin

=========================

Setelah selesai,

kembali ke Screen Saver.

---

# Maintenance

Saat maintenance.

Client mengambil alih layar.

Menampilkan informasi maintenance.

---

# Lifecycle Client

Android TV Boot

↓

Client otomatis berjalan

↓

Koneksi REST

↓

Koneksi WebSocket

↓

Idle

↓

Menunggu Event

↓

ACTIVE

↓

Overlay

↓

Overtime

↓

Idle

↓

Menunggu Event lagi

Client harus selalu hidup selama Android TV menyala.

---

# Permasalahan Saat Ini

Implementasi sekarang menggunakan Flutter sebagai aplikasi fullscreen.

Saat ACTIVE,

Flutter tetap memenuhi seluruh layar.

Akibatnya:

Game tidak terlihat.

YouTube tidak terlihat.

Launcher tidak terlihat.

TV menjadi hitam.

Padahal yang saya inginkan adalah TV tetap terlihat.

---

# Yang Ingin Saya Ketahui

Saya ingin Anda menganalisa requirement ini.

Jangan langsung coding.

Jawab pertanyaan berikut.

## 1

Apakah Flutter murni mampu menjadi overlay di atas aplikasi Android TV lain?

## 2

Jika tidak,

apa batasan Flutter?

## 3

Apakah saya harus menggunakan Android Native?

## 4

Jika ya,

teknologi Android apa yang paling tepat?

Contoh:

- WindowManager
- TYPE_APPLICATION_OVERLAY
- Accessibility Overlay
- Foreground Service
- Overlay Service
- MediaSession
- Picture in Picture
- atau solusi lain yang lebih tepat.

## 5

Bagaimana arsitektur terbaik agar Flutter tetap digunakan sebagai UI,

tetapi Android Native menangani overlay?

## 6

Bagaimana komunikasi terbaik antara:

Flutter

Android Native

REST API

WebSocket

agar sistem tetap bersih dan mudah di-maintain?

## 7

Apakah requirement saya realistis untuk Android TV?

Jika ada requirement yang tidak mungkin dilakukan,

jelaskan alasannya secara teknis.

---

# Yang Saya Inginkan Dari Anda

Jangan langsung membuat kode.

Lakukan langkah berikut.

1. Analisa requirement.

2. Analisa arsitektur saat ini.

3. Jelaskan mengapa implementasi sekarang salah.

4. Jelaskan keterbatasan Flutter.

5. Jelaskan solusi Android Native yang paling tepat.

6. Gambarkan arsitektur baru.

7. Baru setelah arsitektur disepakati, lakukan refactor kode.