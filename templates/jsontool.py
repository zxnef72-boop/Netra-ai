#!/usr/bin/env python3
"""JSON tool: pretty print, minify, atau validasi."""
import json
import sys

def main():
    if len(sys.argv) < 2:
        print("Pakai: python jsontool.py <file.json> [pretty|minify|validate]")
        sys.exit(1)
    path = sys.argv[1]
    mode = sys.argv[2] if len(sys.argv) > 2 else "pretty"
    try:
        with open(path) as f:
            data = json.load(f)
    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)
    if mode == "pretty":
        print(json.dumps(data, indent=2, ensure_ascii=False))
    elif mode == "minify":
        print(json.dumps(data, separators=(",", ":"), ensure_ascii=False))
    elif mode == "validate":
        print(f"✓ Valid JSON ({len(str(data))} byte)")
    else:
        print(f"Mode gak dikenal: {mode}")

if __name__ == "__main__":
    main()
