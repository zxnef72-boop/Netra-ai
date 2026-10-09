# Tutorial: Bikin APK dari Netra

Netra bisa bikin APK dengan 2 cara: dari HP langsung, atau dari cloud GitHub.

## Cara 1: Build dari HP (Termux)

Buka TUI Netra:

    ~/netra-ai-bin

Di dalam TUI:

    /build apk https://example.com ContohApp

Tunggu 30-90 detik. Output:

    Building APK: ContohApp
    URL : https://example.com
    Versi: 1.0.0 (code 10000)
    Icon: default (generate dari inisial)

      -> prepare   ok
      -> icon      ok
      -> java      ok
      -> manifest  ok
      -> styles    ok
      -> compile   ok
      -> dex       ok
      -> package   ok
      -> sign      ok

    APK siap!
    /sdcard/NetRatest1/apk-output/ContohApp.apk (33 KB)

APK otomatis tersimpan di folder output. Buka file manager, tap APK, install.

## Cara 2: Build dari Cloud (GitHub Actions)

Gak butuh PC, gak butuh install apapun. Gratis, unlimited untuk repo public.

Langkah:

1. Buka repo Netra di GitHub
2. Klik tab Actions
3. Pilih workflow Build APK
4. Klik Run workflow
5. Isi URL dan nama app
6. Klik Run
7. Tunggu 1-5 menit
8. Klik run yang ada centang hijau
9. Scroll ke bawah, cari Artifacts
10. Download apk-NamaApp
11. Extract zip, install APK

## Kustomisasi

Bikin APK fullscreen dari HTML lokal:

    /build html ~/project-web ContohApp --fullscreen

Bikin APK dengan versi spesifik:

    /build apk https://example.com ContohApp@2.5.1

Format versi: X.Y.Z (contoh 1.2.3, 2.5.1)
Version code otomatis dihitung: major*10000 + minor*100 + patch

## Tips

Bikin icon sendiri: taruh PNG di ~/.netra-ai/icon.png
Netra otomatis pakai icon itu (bukan generate inisial)

Ukuran APK normal: 25-100 KB

## Disclaimer

Contoh URL (example.com) hanya untuk demonstrasi.
Gunakan tool ini secara etis dan legal.
