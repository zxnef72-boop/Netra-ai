# Netra CLI

Netra CLI adalah toolkit berbasis terminal yang menggabungkan fitur AI, OSINT, pengelolaan file, plugin Lua, dan berbagai alat builder dalam satu aplikasi.

Netra dikembangkan menggunakan Go dan Lua. Aplikasi ini ditujukan untuk beberapa platform, termasuk Android melalui Termux, Linux, Windows, dan macOS. Fitur tertentu mungkin memerlukan program atau dependensi tambahan sesuai sistem operasi yang digunakan.

## Fitur Utama

### AI dan percakapan
- Eliza: mode percakapan offline yang dapat digunakan tanpa API key.
- Dukungan beberapa provider AI, termasuk Gemini, Groq, OpenRouter, dan Ollama, sesuai konfigurasi.
- Sistem plugin Lua untuk menambahkan kemampuan baru.
- Persistent memory dan personality system.

### Agent
Agent dapat membantu mengerjakan tugas di dalam workspace, seperti membaca dan menulis file, melihat isi direktori, serta menjalankan perintah.

Perintah yang tersedia:
- `/agent <prompt>` — memulai tugas agent.
- `/agent-unsafe on` — mengaktifkan izin operasi tulis dan eksekusi perintah.

Agent menggunakan workspace berikut secara default:
- Termux: `/sdcard/AiNef`
- Platform lain: `~/netra-projects`

Lokasi workspace dapat diubah melalui variabel lingkungan `NETRA_WORKSPACE`.

Agent memiliki pembatasan output tool dan mekanisme penanganan rate limit. Gunakan izin eksekusi perintah dengan hati-hati, terutama ketika bekerja pada file penting.

### Builder
Netra menyediakan beberapa pilihan builder:

| Perintah | Fungsi |
|---|---|
| `/build apk <url> <nama>` | Membuat APK dari URL |
| `/build apkfast <url> <nama>` | Membuat APK menggunakan template |
| `/build html <folder> <nama>` | Membuat APK dari folder HTML lokal |
| `/build native <data.json> <nama>` | Membuat APK native dari data JSON |
| `/build exe <folder> <nama>` | Membuat installer Windows |
| `/build exe-win <folder> <nama>` | Membuat installer Windows |
| `/build exe-portable <folder> <nama>` | Membuat EXE portable |
| `/build portable <folder> <nama>` | Alias untuk `exe-portable` |

Mode `--fullscreen` tersedia untuk builder APK dari URL dan folder HTML.

### APK dari HTML
Perintah `/build html` membuat APK dari folder HTML lokal tanpa perlu menjalankan proses kompilasi Java pada saat build. File web dimasukkan ke dalam aplikasi melalui template APK yang sudah disiapkan.

Folder sumber harus memiliki `index.html` atau `index.htm`. Mode fullscreen juga tersedia.

Beberapa API pemilih file dan folder pada browser mungkin tidak didukung oleh WebView Android.

### EXE portable
Perintah `/build exe-portable` mengemas aplikasi HTML menjadi satu file EXE Windows.

Saat dijalankan, aplikasi mengekstrak file web ke direktori sementara Windows, lalu membukanya menggunakan Edge atau Chrome dalam mode aplikasi.

Browser yang didukung harus sudah terpasang. WebView2 belum menjadi bagian dari implementasi ini.

Output secara default disimpan di:
`~/netra-projects/exe-output/`

Lokasi workspace dapat disesuaikan melalui `NETRA_WORKSPACE`.

### OSINT dan utilitas
- `/apkscan <file.apk>` — memeriksa file APK.
- `/sandbox <perintah>` — menjalankan perintah melalui backend sandbox yang tersedia.
- `/lua <file.lua>` — menjalankan file Lua.
- `/baca <url>` — membaca konten dari URL.
- `/search <query>` — melakukan pencarian.

Gunakan fitur OSINT hanya pada file, sistem, dan layanan yang memang boleh kamu periksa.

### HTML Toolkit
HTML Toolkit menyediakan beberapa mode pengolahan halaman web:

- `html tema <url>` — mengambil dan membersihkan HTML.
- `html asli <url>` — mengambil HTML asli dan menambahkan base URL.
- `html link <url>` — mengambil daftar tautan.
- `html info <url>` — mengambil metadata halaman.
- `html offline <url>` — membuat salinan satu halaman untuk penggunaan offline.
- `html crawl <url>` — mengambil beberapa halaman dalam batas yang ditentukan aplikasi.

Output HTML Toolkit secara default disimpan di `/sdcard/AiNef/` pada lingkungan yang menggunakan lokasi tersebut.

### GitHub
- `github cari <keyword>` — mencari repositori.
- `github ambil <owner>/<repo>/<path>` — mengambil isi file dari repositori.

### Direct Handlers
Netra juga menyediakan beberapa perintah berbasis bahasa alami, antara lain:
- Informasi harga Bitcoin dan aset kripto.
- Informasi waktu di kota tertentu.
- Konversi mata uang.
- Pencarian arti dan sinonim kata.
- Kutipan motivasi.
- Penjelasan topik melalui Wikipedia.

Ketersediaan beberapa layanan bergantung pada koneksi internet dan layanan pihak ketiga.

### Ndock
Ndock menyediakan perintah untuk mengelola lingkungan kerja berbasis sistem yang tersedia pada platform pengguna.

Perintah yang didokumentasikan meliputi:
- `ndock pull <nama> <url>`
- `ndock run <nama> <perintah>`
- `ndock ps`
- `ndock logs`
- `ndock inspect`
- `ndock rm`

Backend dan dependensinya mengikuti implementasi serta dukungan platform.

### Linkfinder
Gunakan `/linkfinder <file|url>` untuk mengambil URL atau endpoint dari sumber yang diberikan.

## Sandbox

Netra memilih backend sandbox berdasarkan sistem operasi dan ketersediaan program pendukung.

| Platform | Backend |
|---|---|
| Linux | Bubblewrap sebagai pilihan utama |
| Linux / macOS | Docker sebagai alternatif jika tersedia dan siap digunakan |
| Termux | Proot-Distro melalui mode fallback yang diizinkan |
| Windows | WSL melalui mode fallback yang diizinkan |

Bubblewrap dan Docker menyediakan pembatasan isolasi yang berbeda. Proot-Distro dan WSL bukan container khusus Netra dan tidak memberikan isolasi kernel seperti container atau namespace Linux.

Jika backend yang memenuhi syarat tidak tersedia, Netra tidak akan menjalankan perintah sebagai shell host biasa.

## Instalasi

Netra membutuhkan Go **1.25 atau lebih baru** untuk proses kompilasi.

### Termux (Android)

Pasang dependensi dasar:

```bash
pkg update
pkg install golang git proot-distro
```

Unduh dan kompilasi Netra:

```bash
git clone https://github.com/zxnef72-boop/netra-ai.git
cd netra-ai
go build -o netra-ai-bin .
./netra-ai-bin
```

### Linux, macOS, dan WSL

Pastikan Go 1.25 atau lebih baru sudah terpasang.

```bash
git clone https://github.com/zxnef72-boop/netra-ai.git
cd netra-ai
go build -o netra-ai .
./netra-ai
```

Beberapa fitur memerlukan program tambahan. Contohnya, sandbox Linux menggunakan Bubblewrap sebagai backend utama dan dapat menggunakan Docker jika memenuhi persyaratan.

### Windows

Pastikan Go 1.25 atau lebih baru sudah terpasang.

```powershell
git clone https://github.com/zxnef72-boop/netra-ai.git
cd netra-ai
go build -o netra-ai.exe .
.\netra-ai.exe
```

Panduan instalasi per platform tersedia di [Tutorial Instalasi](docs/TUTORIAL-INSTALL.md).

## Konfigurasi Provider AI

Penggunaan provider AI online bersifat opsional. Netra dapat dijalankan menggunakan Eliza tanpa API key.

Konfigurasi provider disimpan dalam `config.json`. Contoh format tersedia di `config.example.json`.

Jangan memasukkan API key asli ke repositori atau membagikannya kepada orang lain. Simpan konfigurasi pribadi secara lokal.

Untuk melihat provider yang tersedia:

```text
/vendors
```

Untuk memilih provider:

```text
/vendor
```

Kamu juga dapat memilih provider berdasarkan nomor atau nama, misalnya `/vendor 1`.

## Contoh Penggunaan

```text
harga bitcoin
jam di tokyo
arti kata fotosintesis
/build html ~/project MyApp --fullscreen
/build exe-portable ~/project MyApp
/sandbox pwd
/sandbox ls -la
/baca https://id.wikipedia.org/wiki/Fotosintesis
```

## Privasi

- Percakapan dengan provider AI online dikirim ke layanan provider yang dipilih.
- API key disimpan dalam konfigurasi lokal.
- Eliza berjalan secara offline.
- Periksa isi file sebelum membagikannya atau menjalankan perintah yang dapat mengubah data.

## Dokumentasi

- [Dokumentasi Utama](DOKUMENTASI.md)
- [Tutorial Instalasi](docs/TUTORIAL-INSTALL.md)
- [Tutorial Membuat Plugin](docs/TUTORIAL-BIKIN-PLUGIN.md)
- [Tutorial Build APK](docs/TUTORIAL-BUILD-APK.md)

## Lisensi

MIT
