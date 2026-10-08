#!/usr/bin/env python3
"""Domain Recon — HTTP header + robots + DNS dasar
Pakai: python3 domain-recon.py <domain>"""
import sys, socket, urllib.request

def main():
    if len(sys.argv) < 2:
        print("Pakai: python3 domain-recon.py <domain>")
        return
    d = sys.argv[1].replace("https://","").replace("http://","").rstrip("/")
    print(f"=== Recon: {d} ===\n")

    # 1. Resolve
    try:
        ip = socket.gethostbyname(d)
        print(f"[1] IP: {ip}")
    except Exception as e:
        print(f"[1] Gagal resolve: {e}")
        return

    # 2. HTTP header
    print("\n[2] HTTP Headers (port 80):")
    try:
        req = urllib.request.Request(f"http://{d}", method="HEAD",
                                     headers={"User-Agent": "Mozilla/5.0"})
        with urllib.request.urlopen(req, timeout=8) as r:
            for k, v in r.headers.items():
                if k.lower() in ("server","x-powered-by","content-type","location") or k.lower().startswith("x-"):
                    print(f"    {k}: {v}")
    except Exception as e:
        print(f"    Error: {e}")

    # 3. Robots
    print("\n[3] Robots.txt:")
    try:
        with urllib.request.urlopen(f"http://{d}/robots.txt", timeout=8) as r:
            print("   " + r.read(500).decode("utf-8", "ignore").replace("\n","\n   "))
    except Exception:
        print("    Gak ada")

if __name__ == "__main__":
    main()
