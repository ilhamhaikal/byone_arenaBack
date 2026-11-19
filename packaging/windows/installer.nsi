; =============================================================================
; Installer Byone Arena Server (NSIS)
; Dibangun dari Linux dengan: makensis -DPAYLOAD=... -DOUTFILE=... installer.nsi
; =============================================================================

Unicode true
SetCompressor /SOLID lzma

!ifndef VERSION
  !define VERSION "1.0.0"
!endif
!ifndef PAYLOAD
  !error "PAYLOAD belum di-set (gunakan -DPAYLOAD=/path/ke/payload)"
!endif
!ifndef OUTFILE
  !define OUTFILE "byone-arena-setup.exe"
!endif

!define APPNAME     "Byone Arena"
!define APPDESC     "Server & Database Byone Arena"
!define COMPANY     "Byone Arena"
!define REGKEY      "Software\ByoneArena"
!define UNINSTKEY   "Software\Microsoft\Windows\CurrentVersion\Uninstall\ByoneArena"
!define SERVER_EXE  "byone-server.exe"
!define PORT        "8080"

Name "${APPNAME} ${VERSION}"
OutFile "${OUTFILE}"
InstallDir "$PROGRAMFILES64\${APPNAME}"
InstallDirRegKey HKLM "${REGKEY}" "InstallDir"
RequestExecutionLevel admin
BrandingText "${APPNAME} ${VERSION}"

Icon "app.ico"
UninstallIcon "app.ico"

VIProductVersion "${VERSION}.0"
VIAddVersionKey /LANG=1033 "ProductName"     "${APPNAME}"
VIAddVersionKey /LANG=1033 "CompanyName"     "${COMPANY}"
VIAddVersionKey /LANG=1033 "FileDescription" "${APPDESC}"
VIAddVersionKey /LANG=1033 "FileVersion"     "${VERSION}"
VIAddVersionKey /LANG=1033 "ProductVersion"  "${VERSION}"
VIAddVersionKey /LANG=1033 "LegalCopyright"  "Copyright (c) 2026 ${COMPANY}"

!include "MUI2.nsh"
!define MUI_ABORTWARNING

!define MUI_WELCOMEPAGE_TITLE "Instalasi ${APPNAME}"
!define MUI_WELCOMEPAGE_TEXT "Installer ini memasang server ${APPNAME}.\r\n\r\n\
Setelah instalasi:\r\n\
1. Salin .env.example menjadi .env, isi DB_USER & DB_PASSWORD.\r\n\
2. Jalankan 'Setup Database' dari Start Menu.\r\n\
3. Server berjalan otomatis saat Windows menyala (opsional)."

!define MUI_FINISHPAGE_TITLE "Instalasi selesai"
!define MUI_FINISHPAGE_TEXT "Server terpasang di:\r\n$INSTDIR\r\n\r\n\
Langkah berikutnya:\r\n\
1. Buka folder tersebut, salin .env.example -> .env, isi DB_USER & DB_PASSWORD.\r\n\
2. Jalankan 'Setup Database' dari Start Menu (sekali saja).\r\n\
3. Jalankan 'Start Server' atau restart PC (jika auto-start dipilih).\r\n\r\n\
Jika Windows menampilkan peringatan 'Unknown publisher', pilih More info -> Run anyway."
!define MUI_FINISHPAGE_SHOWREADME ""
!define MUI_FINISHPAGE_SHOWREADME_NOTCHECKED

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

; ── Komponen utama ──────────────────────────────────────────────────────────
Section "Server Byone Arena (wajib)" SecCore
  SectionIn RO
  SetShellVarContext all
  SetOutPath "$INSTDIR"

  ; File payload (exe, migrations, .env.example, docs, skrip cmd)
  File /r "${PAYLOAD}/*"

  ; Registry & uninstaller
  WriteRegStr HKLM "${REGKEY}" "InstallDir" "$INSTDIR"
  WriteRegStr HKLM "${REGKEY}" "Version" "${VERSION}"
  WriteRegStr HKLM "${UNINSTKEY}" "DisplayName" "${APPNAME}"
  WriteRegStr HKLM "${UNINSTKEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKLM "${UNINSTKEY}" "Publisher" "${COMPANY}"
  WriteRegStr HKLM "${UNINSTKEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKLM "${UNINSTKEY}" "UninstallString" "$INSTDIR\uninstall.exe"
  WriteRegDWORD HKLM "${UNINSTKEY}" "NoModify" 1
  WriteRegDWORD HKLM "${UNINSTKEY}" "NoRepair" 1
  WriteUninstaller "$INSTDIR\uninstall.exe"

  ; Shortcut Start Menu
  CreateDirectory "$SMPROGRAMS\${APPNAME}"
  CreateShortCut "$SMPROGRAMS\${APPNAME}\Start Server.lnk" "$INSTDIR\start-server.cmd" "" "$INSTDIR\${SERVER_EXE}" 0
  CreateShortCut "$SMPROGRAMS\${APPNAME}\Setup Database.lnk" "$INSTDIR\run-migrate.cmd" "" "$INSTDIR\migrate.exe" 0
  CreateShortCut "$SMPROGRAMS\${APPNAME}\Folder Server.lnk" "$INSTDIR"
  CreateShortCut "$SMPROGRAMS\${APPNAME}\Uninstall.lnk" "$INSTDIR\uninstall.exe"

  ; Firewall: izinkan port server
  nsExec::ExecToLog 'netsh advfirewall firewall delete rule name="${APPNAME} Server"'
  nsExec::ExecToLog 'netsh advfirewall firewall add rule name="${APPNAME} Server" dir=in action=allow protocol=TCP localport=${PORT}'
SectionEnd

; ── Auto-start (opsional, terpilih default) ─────────────────────────────────
Section "Jalankan server otomatis saat Windows menyala" SecAuto
  nsExec::ExecToLog 'schtasks /Delete /TN "${APPNAME} Server" /F'
  nsExec::ExecToLog 'schtasks /Create /TN "${APPNAME} Server" /TR "\"$INSTDIR\start-server.cmd\"" /SC ONSTART /RU SYSTEM /RL HIGHEST /F'
SectionEnd

!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SecCore} "Memasang server, migrasi database, dan panduan."
  !insertmacro MUI_DESCRIPTION_TEXT ${SecAuto} "Membuat scheduled task agar server berjalan saat Windows menyala."
!insertmacro MUI_FUNCTION_DESCRIPTION_END

; ── Uninstall ───────────────────────────────────────────────────────────────
Section "Uninstall"
  SetShellVarContext all

  nsExec::ExecToLog 'taskkill /F /IM ${SERVER_EXE}'
  nsExec::ExecToLog 'schtasks /Delete /TN "${APPNAME} Server" /F'
  nsExec::ExecToLog 'netsh advfirewall firewall delete rule name="${APPNAME} Server"'

  Delete "$SMPROGRAMS\${APPNAME}\Start Server.lnk"
  Delete "$SMPROGRAMS\${APPNAME}\Setup Database.lnk"
  Delete "$SMPROGRAMS\${APPNAME}\Folder Server.lnk"
  Delete "$SMPROGRAMS\${APPNAME}\Uninstall.lnk"
  RMDir "$SMPROGRAMS\${APPNAME}"

  ; JWT secret dibuat otomatis oleh server saat pertama jalan.
  Delete "C:\ProgramData\ByoneArena\jwt.secret"
  RMDir "C:\ProgramData\ByoneArena"

  DeleteRegKey HKLM "${UNINSTKEY}"
  DeleteRegKey HKLM "${REGKEY}"

  ; Jangan hapus .env milik pengguna (berisi password database).
  Delete "$INSTDIR\*.exe"
  Delete "$INSTDIR\*.cmd"
  Delete "$INSTDIR\*.md"
  Delete "$INSTDIR\*.txt"
  Delete "$INSTDIR\.env.example"
  RMDir /r "$INSTDIR\migrations"
  RMDir /r "$INSTDIR\docs"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
SectionEnd
