# Netra CLI

Toolkit OSINT + AI + builder yang jalan di terminal. Didesain buat HP (Termux) tapi bisa cross-platform.

Dibuat dengan Go (mesin) + Lua (otak). Ringan, cepat, satu binary.

## Fitur

Builder:
- /build apk <url> <nama>
- /build html <folder> <nama> [--fullscreen]
- /build native <data.json> <nama>
- /build exe <folder> <nama>
- /build exe-win <folder> <nama>

OSINT:
- /apkscan <file.apk>
- /sandbox <cmd>
- /lua <file.lua>
- /baca <url>
- /search <query>

AI:
- Eliza mode (offline, tanpa API)
- Plugin system Lua (natural language)
- Multi-provider (Gemini, Groq, OpenRouter, Ollama)

TUI:
- File manager (Ctrl+B)
- Persistent memory
- Personality system

## Instalasi (Termux)

    pkg install golang git proot-distro
    git clone https://github.com/zxnef72-boop/netra-ai.git
    cd netra-ai
    go build -o netra-ai-bin .
    ./netra-ai-bin

## Setup AI (opsional)

Isi API key di config.json. Provider yang didukung:
- Gemini (gratis) - ai.google.dev
- Groq (gratis) - console.groq.com
- OpenRouter - openrouter.ai
- Ollama (lokal) - ollama.com

## Contoh Pakai

    check crypto btc
    harga ethereum
    /build html ~/project MyApp --fullscreen
    /sandbox nmap -F 192.168.1.1
    /baca fotosintesis id.wikipedia.org

## Privacy

Chat dengan AI dikirim ke provider yang kamu pilih. API key disimpan lokal. Eliza mode offline 100% lokal.

## Lisensi

MIT
