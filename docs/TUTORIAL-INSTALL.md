# Tutorial: Install Netra di Semua OS

Netra bisa jalan di Termux (Android), Linux, Windows, macOS.

## Termux (Android)

### 1. Install Termux

Download dari F-Droid (jangan Play Store, versi lama):
https://f-droid.org/en/packages/com.termux/

### 2. Install dependencies

    pkg update
    pkg install golang git proot-distro

### 3. Clone + build

    git clone https://github.com/zxnef72-boop/netra-ai.git
    cd netra-ai
    go build -o ~/netra-ai-bin .

### 4. Jalanin

    ~/netra-ai-bin

Fitur yang jalan di Termux: SEMUA.

## Linux (Ubuntu / Debian / Arch / Fedora)

### 1. Install dependencies

Ubuntu/Debian:

    sudo apt update
    sudo apt install -y golang git docker.io

Arch:

    sudo pacman -S go git docker

Fedora:

    sudo dnf install -y golang git docker

### 2. Aktifin Docker (opsional, buat sandbox)

    sudo systemctl enable --now docker
    sudo usermod -aG docker $USER
    # Logout + login ulang biar group ke-apply

### 3. Clone + build

    git clone https://github.com/zxnef72-boop/netra-ai.git
    cd netra-ai
    go build -o netra-ai .

### 4. Jalanin

    ./netra-ai

Fitur yang jalan: SEMUA.
Sandbox otomatis pakai Docker (bukan proot).

## Windows 10/11

### Cara 1: WSL (rekomendasi)

Install WSL2 + Ubuntu:

    wsl --install

Setelah WSL jalan, di dalam WSL ikuti langkah Linux di atas.

Netra jalan di WSL, sandbox pakai WSL juga.

### Cara 2: Native Windows

Install Go for Windows:
https://go.dev/dl/

Install Git for Windows:
https://git-scm.com/

Buka PowerShell:

    git clone https://github.com/zxnef72-boop/netra-ai.git
    cd netra-ai
    go build -o netra-ai.exe .
    .\netra-ai.exe

Fitur yang jalan: chat, plugin, search, baca, build exe-win.
Fitur yang gak jalan: sandbox, build apk (butuh Linux tools).

### Cara 3: Docker Desktop

Install Docker Desktop for Windows.
Buat folder project, taruh Dockerfile:

    FROM golang:1.22-alpine AS build
    WORKDIR /src
    COPY . .
    RUN go build -o /netra-ai .

    FROM alpine:latest
    RUN apk add --no-cache bash git
    COPY --from=build /netra-ai /usr/local/bin/netra-ai
    ENTRYPOINT ["netra-ai"]

Build + jalanin:

    docker build -t netra-ai .
    docker run -it --rm netra-ai

## macOS (Intel & Apple Silicon)

### 1. Install Homebrew

    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

### 2. Install dependencies

    brew install go git
    brew install --cask docker

### 3. Build

    git clone https://github.com/zxnef72-boop/netra-ai.git
    cd netra-ai
    go build -o netra-ai .

### 4. Jalanin

    ./netra-ai

Fitur yang jalan: SEMUA.
Sandbox otomatis pakai Docker.

## Sandbox per OS

Netra deteksi OS otomatis:

| OS | Backend | Butuh install |
|---|---|---|
| Termux | proot-distro | proot-distro + alpine |
| Linux | Docker | docker.io |
| Windows | WSL | wsl + ubuntu |
| macOS | Docker Desktop | Docker Desktop |
| Fallback | shell logis | - |

Kalau backend gak tersedia, Netra fallback ke "sandbox logis" (cwd dikurung, tanpa isolasi). Header bakal bilang "Isolasi: tidak ada" — jujur.

## Fitur per OS

| Fitur | Termux | Linux | Windows | macOS |
|---|---|---|---|---|
| Chat Eliza | ya | ya | ya | ya |
| Plugin Lua | ya | ya | ya | ya |
| Search + baca | ya | ya | ya | ya |
| Sandbox | ya | ya | WSL | ya |
| Build APK | ya | ya | tidak | tidak |
| Build EXE | ya | ya | ya | ya |
| Build EXE-Win | ya | ya | ya | ya |

Windows bisa build APK kalau pakai WSL + install Android SDK.

## Troubleshooting

### "go: command not found"
Go belum keinstall atau belum masuk PATH.
Cek: go version

### "git: command not found"
Install git dulu.

### "permission denied" pas build
Ganti output ke home:

    go build -o ~/netra-ai .

### Sandbox bilang "Isolasi: tidak ada"
Berarti proot-distro atau Docker gak tersedia.
Install sesuai OS di atas.

### Build APK gagal di Windows
Windows native gak support aapt/dx/javac Linux.
Solusi: pakai WSL atau GitHub Actions.
