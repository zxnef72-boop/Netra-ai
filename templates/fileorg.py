#!/usr/bin/env python3
"""Auto-organize file berdasarkan ekstensi."""
import os
import shutil
from pathlib import Path

CATEGORIES = {
    "Images": [".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg"],
    "Documents": [".pdf", ".docx", ".doc", ".txt", ".md"],
    "Videos": [".mp4", ".mkv", ".avi", ".mov"],
    "Audio": [".mp3", ".wav", ".flac", ".m4a"],
    "Archives": [".zip", ".rar", ".7z", ".tar", ".gz"],
    "Code": [".py", ".go", ".lua", ".js", ".html", ".css"],
}

def organize(folder):
    folder = Path(folder).expanduser()
    if not folder.is_dir():
        print(f"Error: {folder} bukan folder")
        return
    moved = 0
    for f in folder.iterdir():
        if f.is_file():
            ext = f.suffix.lower()
            for cat, exts in CATEGORIES.items():
                if ext in exts:
                    dest = folder / cat
                    dest.mkdir(exist_ok=True)
                    shutil.move(str(f), str(dest / f.name))
                    print(f"  {f.name} → {cat}/")
                    moved += 1
                    break
    print(f"\nSelesai: {moved} file dipindah")

if __name__ == "__main__":
    import sys
    target = sys.argv[1] if len(sys.argv) > 1 else "."
    organize(target)
