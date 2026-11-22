# Netra AI — Dokumentasi

**Author:** Anzar (zxnef72-boop)
**Platform:** Termux (Android) + portable ke Linux/Windows/Mac
**Bahasa:** Go + Lua
**Repo:** github.com/zxnef72-boop/Netra-ai

---

## Apa itu Netra

Asisten AI portabel yang jalan di HP (Termux) atau laptop.

Dua otak:
- **Eliza** — offline, if-else + rules + Lua (buatan sendiri)
- **Vendor** — online, LLM dari Gemini/Groq/OpenRouter/Ollama

---

## Cara Jalanin

cd ~/netra-ai
go build -o ~/netra-ai-bin .
~/netra-ai-bin

Kalau binary stale:
rm -f ~/netra-ai-bin && go build -o ~/netra-ai-bin .

Kontrol TUI:
- Enter — kirim
- Ctrl+C — keluar
- Ctrl+N — chat baru
- Ctrl+S — history
- Ctrl+B — file manager
- Esc — layer-aware

---

## Commands

Chat:
- /help /new /sessions /clear /model

Provider:
- /vendor — list
- /vendor <nomor|nama> — ganti

Agent:
- /agent <prompt> — jalanin otonom
- /agent-unsafe on — aktifin write+run

File:
- /save /saveall /blocks /copy /cd /zip /unzip /template /project

Lua:
- /lua <file.lua> — jalanin
- /linkfinder <target> — extract URL

Ndock:
- /ndock pull|run|images|ps|logs|inspect|rm

Build:
- /build apk|exe-win|html

Personality:
- /persona [nama|off]

---

## Eliza (Offline)

Pure Go + Lua.

Fitur:
- Greeting, tanya nama
- Mood (sedih/senang/takut/marah)
- Fun: coin, dadu, quote, joke, random
- Curhat: keluarga, teman, kerja, cinta, coding
- Topic memory + context
- Entity recognition
- Lua rules (hot-reload)
- Personality via Lua

Direct Handlers:
- Crypto — CoinGecko
- Waktu kota — Go native
- Kurs — Frankfurter
- Kamus EN — dictionaryapi.dev
- Sinonim — Datamuse
- Quote — ZenQuotes
- Wikipedia — ID + EN fallback

Karakter: "Aku if-else. Aku gak ngerti. Tapi aku dengerin."

---

## Vendor (Online)

Provider:
- Gemini (format khusus)
- Groq (openai-compat)
- OpenRouter (openai-compat)
- Ollama (openai-compat)
- Puter (openai-compat)

Fitur: streaming, auto-switch rate limit, system prompt.

---

## Agent

Tools: read_file, write_file, list_dir, run_command.

Safety:
- Path restriction /sdcard/AiNef
- Unsafe mode /agent-unsafe on
- Loop detection max 8 steps
- Output limit 5-8KB

Progres live: selama agent kerja, chat menampilkan checklist langkah:
spinner = jalan, centang = berhasil (+ ukuran/jumlah baris), silang = gagal (+ alasan), titik = antri.
Event dikirim dari agent.go lewat hook (agent_progress.go), ditangkap di ui_anim.go, digambar di ui_opencode.go.

Requirement: OpenAI-compat provider (bukan eliza-1966).

---

## Ndock

Backend: proot-distro (Termux), chroot/docker (Linux), wsl (Windows).

Command: pull, run, images, ps, logs, inspect, rm.

Fiturs: auto-log rotasi 100KB, cache 1 jam, protect alpine + netra-sandbox.

Path: $PREFIX/var/lib/proot-distro/containers/

---

## HTML Toolkit

- html tema X — fetch + bersihin
- html asli X — fetch mentah + base
- html link X — extract link
- html info X — extract metadata
- html offline X — snapshot 1 file
- html crawl X — multi-page mirror

Output: /sdcard/AiNef/

---

## Build APK dari HTML

    /build html /sdcard/NefPlayer nef-player [--fullscreen]

Folder wajib punya index.html. Semua isi folder (css, js, gambar, subfolder) ikut masuk ke APK.
Tanpa Java: pakai template base.apk (embed), tambah assets/www/, ganti config.json jadi
file:///android_asset/www/index.html, tanda tangan v1 pure Go. Output: /sdcard/NetRatest1/apk-output/

- --fullscreen: ganti tema manifest jadi Theme.NoTitleBar.Fullscreen (status bar hilang)
- Izin READ_MEDIA_AUDIO / READ_EXTERNAL_STORAGE ditambah otomatis; app minta izin audio saat dibuka
- Nama paket sama untuk semua hasil (com.netra.template): memasang dua app saling menimpa
- "Pilih file" di halaman web jalan; "Pilih folder" tidak (tidak didukung WebView)

Update template (sekali, butuh ecj/javac + d8 + zip + android.jar di ~/android-sdk/android.jar):

    bash template-src/build_template.sh ~/netra-ai
    go build -o netra-ai-bin .

---

## GitHub

- github cari <keyword> — cari repo
- github ambil <owner>/<repo>/<path> — fetch raw

---

## Config

File: ~/netra-ai/config.json

Field per provider: name, type, api_key, model, base_url, max_tokens.

Active_provider = index provider aktif (-1 = eliza).

JANGAN commit config.json — udah di .gitignore.

---

## Troubleshooting

Build gagal: rm -f ~/netra-ai-bin && go build.

Vendor gak jawab: cek /vendor, test curl API.

Agent gak bisa write: /agent-unsafe on.

API key bocor: revoke di dashboard, generate baru.

---

Dibuat: 12 Oktober 2026
