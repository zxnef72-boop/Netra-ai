#!/data/data/com.termux/files/usr/bin/bash
# build_template.sh — bikin base.apk baru (dengan pemilih file) dari MainActivity.java.
# Cuma sekali. Setelah itu /build html tetap tanpa Java.
#
# Butuh (sekali pasang):  pkg install ecj d8 zip
# Butuh android.jar di:   ~/android-sdk/android.jar  (atau set ANDROID_JAR)
set -e
cd "$(dirname "$0")"
REPO="${1:-$HOME/netra-ai}"
JAR="${ANDROID_JAR:-$HOME/android-sdk/android.jar}"
[ -f "$JAR" ]            || { echo "android.jar gak ada di $JAR (set ANDROID_JAR=...)"; exit 1; }
[ -f "$REPO/base.apk" ]  || { echo "base.apk gak ada di $REPO"; exit 1; }
command -v zip >/dev/null || { echo "pasang dulu: pkg install zip"; exit 1; }
command -v d8  >/dev/null || { echo "pasang dulu: pkg install d8"; exit 1; }

if   command -v ecj   >/dev/null; then JC="ecj"
elif command -v javac >/dev/null; then JC="javac"
else echo "pasang dulu: pkg install ecj"; exit 1; fi

W="$(mktemp -d)"
mkdir -p "$W/classes" "$W/dex"
echo "[1/4] compile ($JC)"
$JC -source 8 -target 8 -cp "$JAR" -d "$W/classes" MainActivity.java
echo "[2/4] dex (d8)"
d8 --release --min-api 21 --lib "$JAR" --output "$W/dex" $(find "$W/classes" -name '*.class')
[ -f "$W/dex/classes.dex" ] || { echo "d8 gak hasilin classes.dex"; exit 1; }

echo "[3/4] ganti classes2.dex di base.apk"
cp "$REPO/base.apk" "$REPO/base.apk.bak"
cp "$W/dex/classes.dex" "$W/classes2.dex"
zip -q -d "$REPO/base.apk" classes2.dex || true
( cd "$W" && zip -q -j "$REPO/base.apk" classes2.dex )

echo "[4/4] selesai: $REPO/base.apk (backup: base.apk.bak)"
echo "Sekarang: cd $REPO && go build -o netra-ai-bin ."
rm -rf "$W"
