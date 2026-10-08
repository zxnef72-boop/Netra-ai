#!/usr/bin/env python3
"""Sherlock-Lite — cek username di banyak platform. No deps."""
import sys, json, urllib.request, urllib.error, argparse
from concurrent.futures import ThreadPoolExecutor, as_completed

PLATFORMS = {
    "GitHub":"https://github.com/{}",
    "GitLab":"https://gitlab.com/{}",
    "Bitbucket":"https://bitbucket.org/{}/",
    "Twitter/X":"https://twitter.com/{}",
    "Instagram":"https://www.instagram.com/{}/",
    "Facebook":"https://www.facebook.com/{}",
    "Reddit":"https://www.reddit.com/user/{}",
    "YouTube":"https://www.youtube.com/@{}",
    "TikTok":"https://www.tiktok.com/@{}",
    "Pinterest":"https://www.pinterest.com/{}/",
    "Twitch":"https://www.twitch.tv/{}",
    "Steam":"https://steamcommunity.com/id/{}",
    "Spotify":"https://open.spotify.com/user/{}",
    "SoundCloud":"https://soundcloud.com/{}",
    "Medium":"https://medium.com/@{}",
    "Dev.to":"https://dev.to/{}",
    "Hacker News":"https://news.ycombinator.com/user?id={}",
    "Behance":"https://www.behance.net/{}",
    "Dribbble":"https://dribbble.com/{}",
    "Vimeo":"https://vimeo.com/{}",
    "Keybase":"https://keybase.io/{}",
    "Telegram":"https://t.me/{}",
    "VK":"https://vk.com/{}",
    "Tumblr":"https://{}.tumblr.com",
    "WordPress":"https://{}.wordpress.com",
    "About.me":"https://about.me/{}",
    "Gravatar":"https://en.gravatar.com/{}",
    "Patreon":"https://www.patreon.com/{}",
    "Quora":"https://www.quora.com/profile/{}",
    "HackerRank":"https://www.hackerrank.com/{}",
    "LeetCode":"https://leetcode.com/{}/",
    "CodePen":"https://codepen.io/{}",
    "Replit":"https://replit.com/@{}",
    "npm":"https://www.npmjs.com/~{}",
    "PyPI":"https://pypi.org/user/{}/",
    "Docker Hub":"https://hub.docker.com/u/{}",
    "Kaggle":"https://www.kaggle.com/{}",
    "Product Hunt":"https://www.producthunt.com/@{}",
    "Last.fm":"https://www.last.fm/user/{}",
    "Bandcamp":"https://{}.bandcamp.com",
    "ArtStation":"https://www.artstation.com/{}",
    "Itch.io":"https://{}.itch.io",
    "Chess.com":"https://www.chess.com/member/{}",
    "Lichess":"https://lichess.org/@/{}",
    "Letterboxd":"https://letterboxd.com/{}/",
    "Wattpad":"https://www.wattpad.com/user/{}",
    "Linktree":"https://linktr.ee/{}",
    "Substack":"https://{}.substack.com",
    "Figma":"https://www.figma.com/@{}",
    "Hashnode":"https://hashnode.com/@{}",
    "freeCodeCamp":"https://www.freecodecamp.org/{}",
    "TryHackMe":"https://tryhackme.com/p/{}",
    "Codeforces":"https://codeforces.com/profile/{}",
    "HackerEarth":"https://www.hackerearth.com/@{}",
    "Devpost":"https://devpost.com/{}",
    "Replit (alt)":"https://replit.com/@{}",
    "About.me":"https://about.me/{}",
    "Bit.ly":"https://bit.ly/{}",
}

UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0 Safari/537.36"

def check(name, tmpl, user, timeout=8):
    url = tmpl.replace("{}", user)
    try:
        req = urllib.request.Request(url, headers={"User-Agent": UA})
        with urllib.request.urlopen(req, timeout=timeout) as r:
            if r.status != 200:
                return (name, url, False, r.status)
            body = r.read(4096).decode("utf-8", errors="ignore").lower()
            soft = ["page not found", "user not found", "profile not found",
                    "doesn't exist", "tidak ditemukan", "not found",
                    "404", "no longer available"]
            if any(s in body for s in soft):
                return (name, url, False, 200)
            return (name, url, True, 200)
    except urllib.error.HTTPError as e:
        return (name, url, False, e.code)
    except Exception:
        return (name, url, False, 0)

def main():
    p = argparse.ArgumentParser(description="Sherlock-Lite")
    p.add_argument("username")
    p.add_argument("--json", action="store_true")
    p.add_argument("--workers", type=int, default=20)
    p.add_argument("--timeout", type=float, default=8)
    p.add_argument("--quiet", action="store_true")
    a = p.parse_args()
    user = a.username.strip()
    if not user:
        print("Error: username kosong"); sys.exit(1)

    if not a.json:
        print(f"Sherlock-Lite — Target: {user}")
        print(f"Cek {len(PLATFORMS)} platform...\n")

    found = []; nf = []; errs = 0
    with ThreadPoolExecutor(max_workers=a.workers) as ex:
        futs = {ex.submit(check, n, u, user, a.timeout): n for n, u in PLATFORMS.items()}
        done = 0
        for f in as_completed(futs):
            name, url, ok, st = f.result()
            done += 1
            if ok:
                found.append({"platform": name, "url": url})
                if not a.json:
                    print(f"  [✓] {name:20} {url}")
            else:
                nf.append({"platform": name, "url": url, "status": st})
                if st == 0: errs += 1
                if not a.json and not a.quiet:
                    tag = "[!]" if st == 0 else "[-]"
                    print(f"  {tag} {name:20} ({st})")

    if a.json:
        print(json.dumps({"username": user, "total": len(PLATFORMS),
            "found_count": len(found), "found": found,
            "not_found": nf, "errors": errs}, indent=2, ensure_ascii=False))
        return

    print(f"\n=== KETEMU di {len(found)}/{len(PLATFORMS)} platform ===")
    if errs: print(f"({errs} gagal konek)")
    if found:
        print("\nLink buat dicek manual:")
        for f in found:
            print(f"  - {f['platform']}: {f['url']}")
    else:
        print("Gak ketemu di platform manapun.")

if __name__ == "__main__":
    main()
