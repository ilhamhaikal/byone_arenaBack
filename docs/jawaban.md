Ya, sebenarnya masalah utamanya bukan pada widget Flutter, tetapi pada arsitektur aplikasi.

Anda bisa menjelaskan seperti ini:

Permasalahan Saat Ini

Saat ini aplikasi Client Android TV dibuat sebagai FlutterActivity (fullscreen Activity).

Konsekuensinya, ketika aplikasi dijalankan Android akan membuat sebuah Window dan Surface milik Flutter yang menempati seluruh layar TV.

Urutannya menjadi seperti ini:

Google TV Launcher
        │
        ▼
FlutterActivity dibuka
        │
        ▼
Flutter membuat Surface fullscreen
        │
        ▼
Surface Flutter berada di atas semua aplikasi lain

Akibatnya:

Launcher Google TV tertutup.
Game yang sedang berjalan tertutup.
YouTube tertutup.
Emulator tertutup.
HDMI tertutup.

Walaupun di Flutter hanya menampilkan:

return const SizedBox();

atau

Container()

atau bahkan widget kosong,

Surface Flutter tetap memenuhi seluruh layar, sehingga yang terlihat hanyalah layar hitam.

Dengan kata lain, layar hitam bukan berasal dari widget, melainkan dari Surface fullscreen milik FlutterActivity.

Yang Seharusnya Terjadi

Client Android TV bukanlah aplikasi utama TV.

Client hanya bertugas sebagai lapisan informasi (overlay).

Ketika pemain sedang menggunakan:

Game
YouTube
Emulator
Launcher
HDMI

maka tampilan tersebut harus tetap terlihat.

Client hanya boleh menampilkan informasi seperti:

indikator LIVE
sisa waktu
warning 5 menit
countdown 10 detik
notifikasi dari server

tanpa mengambil alih layar TV.

Arsitektur yang diinginkan adalah:

Game / YouTube / Launcher
        │
        ▼
Tetap tampil normal
        │
        ▼
Overlay Byone Arena muncul di atasnya

+ LIVE
+ Countdown
+ Warning
+ Notification

Bukan seperti sekarang:

FlutterActivity fullscreen

↓

Layar hitam

↓

Widget Flutter
Mengapa FlutterActivity Tidak Cocok

FlutterActivity memang dirancang sebagai aplikasi utama.

Setiap FlutterActivity selalu memiliki:

Window

↓

Surface

↓

Flutter UI

Surface tersebut selalu fullscreen.

Artinya, FlutterActivity memang akan menggantikan tampilan aplikasi yang sebelumnya berada di layar.

Karena itu FlutterActivity tidak cocok untuk kebutuhan seperti:

floating overlay
heads-up display
HUD
timer di atas game
notifikasi di atas aplikasi lain
Solusi Arsitektur yang Benar

Untuk requirement Byone Arena, Client tidak seharusnya dijalankan sebagai FlutterActivity fullscreen.

Yang lebih sesuai adalah menggunakan Overlay Window melalui Android Native, misalnya dengan WindowManager dan TYPE_APPLICATION_OVERLAY (sesuai izin dan dukungan perangkat Android TV), atau mekanisme overlay lain yang memang didukung platform.

Arsitekturnya menjadi:

Android TV

↓

Foreground Service

↓

WebSocket

↓

WindowManager Overlay

↓

Flutter View / Native View

↓

Overlay kecil di atas Game / YouTube / Launcher

Dengan pendekatan ini:

Game tetap berjalan.
YouTube tetap berjalan.
Launcher tetap terlihat.
HDMI tetap terlihat.
Client hanya menggambar overlay informasi tanpa mengambil alih seluruh layar.
Kesimpulan

Masalah yang terjadi saat ini bukan karena kode UI Flutter, melainkan karena Client dibangun sebagai FlutterActivity fullscreen. Selama Client tetap menggunakan FlutterActivity sebagai tampilan utama, Android akan selalu membuat surface fullscreen yang menutupi seluruh layar TV, sehingga hasilnya adalah layar hitam ketika tidak ada konten yang digambar.

Jika tujuan Client adalah hanya menampilkan informasi seperti LIVE, sisa waktu, countdown, warning, dan notifikasi di atas aplikasi lain, maka arsitektur harus diubah dari fullscreen Activity menjadi overlay window yang berjalan melalui komponen Android Native dan menerima event dari backend (WebSocket/REST). Dengan demikian, Client berfungsi sebagai lapisan informasi, bukan sebagai aplikasi yang menggantikan tampilan TV.

Menurut saya screen saver tetap harus fullscreen. Justru itu yang membedakan state Idle dan Active.

Saya akan menjawab seperti ini kepada Claude:

Jawaban Requirement

Ya, screen saver Starfield tetap harus tampil fullscreen saat state idle.

Hal ini karena pada kondisi idle tidak ada aplikasi yang sedang digunakan oleh pemain.

Artinya Client boleh mengambil alih tampilan TV untuk menampilkan screen saver.

Flow yang diinginkan adalah:

TV Boot
      │
      ▼
Client berjalan
      │
      ▼
Idle
      │
      ▼
Screen Saver fullscreen
      │
      ▼
Admin membuat sesi
      │
      ▼
Client menerima event WebSocket
      │
      ▼
Screen Saver ditutup
      │
      ▼
TV kembali menampilkan Launcher / Game / YouTube / HDMI
      │
      ▼
Overlay LIVE muncul
      │
      ▼
Warning
      │
      ▼
Countdown
      │
      ▼
Overtime
      │
      ▼
Fullscreen "WAKTU HABIS"
      │
      ▼
Kembali ke Screen Saver fullscreen
Requirement setiap state
Idle

Gunakan fullscreen.

Yang ditampilkan:

Starfield Screen Saver
Jam
Logo
Informasi Arena
Promo

Pada state ini memang diperbolehkan mengambil alih layar TV karena console sedang tidak dipakai.

Active

Jangan fullscreen.

Saat status berubah menjadi active:

Tutup screen saver.
Jangan tampilkan FlutterActivity fullscreen.
Jangan tampilkan background hitam.
Jangan mengambil alih layar TV.

Yang harus terlihat adalah aplikasi yang sedang digunakan pemain, misalnya:

Game
YouTube
Emulator
Launcher
HDMI

Client hanya menampilkan overlay:

LIVE
Remaining Time
Warning 5 menit
Countdown 10 detik
Notifikasi
Overtime

Kembali fullscreen.

Tampilkan:

=====================

WAKTU HABIS

Silakan hubungi admin

=====================

Setelah admin mengakhiri sesi:

Overtime
    │
    ▼
Idle
    │
    ▼
Screen Saver fullscreen
Maintenance

Fullscreen.

Tampilkan informasi bahwa console sedang maintenance.

Kesimpulan

Jadi hanya ada dua mode tampilan:

Mode 1 — Fullscreen

Digunakan saat:

Idle
Loading
Overtime
Maintenance

Karena memang tidak ada aplikasi pemain yang perlu ditampilkan.

Mode 2 — Overlay

Digunakan saat:

Active

Karena pemain sedang menggunakan console, sehingga Client tidak boleh menutupi tampilan TV.

Menurut saya ini adalah arsitektur yang paling konsisten untuk sistem rental console seperti Byone Arena:

Idle → TV menjadi media promosi (screen saver fullscreen).
Active → TV menjadi milik pemain, Client hanya sebagai overlay informasi.
Overtime/Maintenance → Client kembali mengambil alih layar karena perlu memberikan informasi penting kepada pemain.