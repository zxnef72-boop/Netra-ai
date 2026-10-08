#!/usr/bin/env python3
"""Email Check — Gravatar + breach links
Pakai: python3 email-check.py <email>"""
import sys, hashlib, urllib.request

def main():
    if len(sys.argv) < 2:
        print("Pakai: python3 email-check.py <email>")
        return
    email = sys.argv[1].lower().strip()
    h = hashlib.md5(email.encode()).hexdigest()
    print(f"=== Email: {email} ===\n")

    # Gravatar
    url = f"https://www.gravatar.com/avatar/{h}?d=404"
    req = urllib.request.Request(url, method="HEAD")
    try:
        with urllib.request.urlopen(req, timeout=8) as r:
            if r.status == 200:
                print(f"  [+] Gravatar ADA")
                print(f"      Foto : https://www.gravatar.com/avatar/{h}")
                print(f"      Profil: https://www.gravatar.com/{h}")
    except Exception:
        print("  [-] Gravatar gak ada")

    # Breach links
    print("\n=== Cek Kebocoran ===")
    print(f"  HaveIBeenPwned : https://haveibeenpwned.com/account/{email}")
    print(f"  Dehashed       : https://dehashed.com/search?query={email}")
    print(f"  IntelX         : https://intelx.io/?s={email}")

if __name__ == "__main__":
    main()
