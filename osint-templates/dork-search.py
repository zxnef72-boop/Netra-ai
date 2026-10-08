#!/usr/bin/env python3
"""Dork Search — generate link Google dork
Pakai: python3 dork-search.py <target>"""
import sys, urllib.parse

PATTERNS = [
    '"{t}"',
    '"{t}" site:facebook.com',
    '"{t}" site:instagram.com',
    '"{t}" site:twitter.com',
    '"{t}" site:linkedin.com',
    '"{t}" site:github.com',
    '"{t}" site:t.me',
    '"{t}" site:pastebin.com',
    '"{t}" site:reddit.com',
    '"{t}" filetype:pdf',
    '"{t}" filetype:xlsx',
    '"{t}" filetype:sql',
    '"{t}" intext:password',
    '"{t}" "leaked"',
    '"{t}" "database"',
    '"{t}" "registered"',
    '"{t}" "contact"',
    '"{t}" "admin"',
]

def main():
    if len(sys.argv) < 2:
        print("Pakai: python3 dork-search.py <target>")
        return
    t = sys.argv[1]
    print(f"=== Dork untuk: {t} ===\n")
    for i, p in enumerate(PATTERNS, 1):
        q = p.format(t=t)
        enc = urllib.parse.quote(q)
        print(f"{i:2d}. https://www.google.com/search?q={enc}")

if __name__ == "__main__":
    main()
