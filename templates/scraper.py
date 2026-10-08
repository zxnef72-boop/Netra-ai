#!/usr/bin/env python3
"""Web scraper sederhana - ambil judul + link dari halaman."""
import urllib.request
import re
import sys

def fetch(url):
    req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
    with urllib.request.urlopen(req, timeout=10) as r:
        return r.read().decode("utf-8", errors="ignore")

def scrape(url):
    html = fetch(url)
    title = re.search(r"<title>(.*?)</title>", html, re.I | re.S)
    print(f"URL: {url}")
    if title:
        print(f"Judul: {title.group(1).strip()}")
    links = re.findall(r'href="(https?://[^"]+)"', html)
    print(f"\nTotal link: {len(links)}")
    for l in links[:20]:
        print(f"  {l}")
    return links

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Pakai: python scraper.py <url>")
        sys.exit(1)
    scrape(sys.argv[1])
