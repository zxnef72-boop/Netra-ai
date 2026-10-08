#!/usr/bin/env python3
"""IP Info — pakai ipinfo.io (gratis 50k req/bulan)
Pakai: python3 ip-info.py <ip>"""
import sys, json, urllib.request

def main():
    if len(sys.argv) < 2:
        print("Pakai: python3 ip-info.py <ip>")
        return
    ip = sys.argv[1]
    print(f"=== IP Info: {ip} ===\n")
    try:
        with urllib.request.urlopen(f"https://ipinfo.io/{ip}/json", timeout=10) as r:
            data = json.load(r)
        for k, v in data.items():
            print(f"  {k:10s}: {v}")
    except Exception as e:
        print(f"Error: {e}")

if __name__ == "__main__":
    main()
