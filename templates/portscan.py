#!/usr/bin/env python3
"""Netra Port Scanner — gaya Nmap.
Fitur: banner grab, OS detect (TTL), timing, tabel rapi.
"""
import socket
import sys
import time
import subprocess
import re
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime

# ─── Service name map (port → nama) ──────────────────────────
SERVICES = {
    21: "ftp", 22: "ssh", 23: "telnet", 25: "smtp", 53: "domain",
    80: "http", 110: "pop3", 143: "imap", 443: "https", 445: "microsoft-ds",
    587: "submission", 993: "imaps", 995: "pop3s",
    1433: "ms-sql", 1521: "oracle", 3306: "mysql", 3389: "ms-wbt",
    5432: "postgresql", 5900: "vnc", 6379: "redis",
    8080: "http-proxy", 8443: "https-alt", 27017: "mongodb",
}

# ─── Probe payload per service ────────────────────────────────
def probe_payload(port, host):
    if port in (80, 8000, 8080, 8888):
        return f"GET / HTTP/1.0\r\nHost: {host}\r\nUser-Agent: netra/1.0\r\n\r\n".encode()
    if port in (443, 8443):
        return None  # TLS — skip probe, cuma connect
    if port == 25:
        return b"EHLO netra.local\r\n"
    if port == 6379:
        return b"INFO server\r\n"
    if port == 110:
        return b"USER test\r\n"
    return b""


def scan_port(host, port, timeout=0.8):
    """Return (port, open, banner, rtt_ms)."""
    t0 = time.time()
    try:
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
            s.settimeout(timeout)
            if s.connect_ex((host, port)) != 0:
                return (port, False, "", 0)
            rtt = (time.time() - t0) * 1000

            # Grab banner
            banner = ""
            try:
                payload = probe_payload(port, host)
                if payload:
                    s.sendall(payload)
                s.settimeout(1.0)
                data = s.recv(512)
                if data:
                    banner = data.decode(errors="replace").split("\r\n")[0][:60]
            except Exception:
                pass

            return (port, True, banner, rtt)
    except Exception:
        return (port, False, "", 0)


def detect_os(host):
    """Tebak OS dari TTL ping."""
    try:
        result = subprocess.run(
            ["ping", "-c", "1", "-W", "2", host],
            capture_output=True, text=True, timeout=4
        )
        m = re.search(r"ttl[=\s]+(\d+)", result.stdout, re.IGNORECASE)
        if not m:
            return None, None
        ttl = int(m.group(1))
        # Cari latency juga
        lat = re.search(r"time[=<]([\d.]+)", result.stdout)
        latency = float(lat.group(1)) if lat else None

        if ttl > 128:
            return "Network device (TTL=255)", latency
        elif ttl > 64:
            return "Windows (TTL=128)", latency
        else:
            return "Linux/Unix (TTL=64)", latency
    except Exception:
        return None, None


def resolve(host):
    """Hostname → IP. Return (host, ip) atau (host, host)."""
    try:
        return host, socket.gethostbyname(host)
    except socket.gaierror:
        return host, None


def fmt_banner(port, banner):
    """Format service version dari banner."""
    if not banner:
        return SERVICES.get(port, "unknown")
    b_lower = banner.lower()
    # Coba ekstrak nama server + versi
    if "nginx" in b_lower:
        m = re.search(r"nginx/([\d.]+)", b_lower)
        return f"nginx {m.group(1) if m else ''}".strip()
    if "apache" in b_lower:
        m = re.search(r"apache/([\d.]+)", b_lower)
        return f"Apache {m.group(1) if m else ''}".strip()
    if "openssh" in b_lower:
        m = re.search(r"openssh[_/]([\d.p]+)", b_lower)
        return f"OpenSSH {m.group(1) if m else ''}".strip()
    if "cloudflare" in b_lower:
        return "cloudflare"
    if "redis_version" in b_lower:
        m = re.search(r"redis_version:([\d.]+)", banner)
        return f"Redis {m.group(1) if m else ''}".strip()
    # Fallback: potong banner
    return banner[:40]


def main():
    if len(sys.argv) < 2:
        print("Pakai: python portscan.py <host> [start] [end]")
        print("       python portscan.py example.com 1 1024")
        print("       python portscan.py example.com top")
        sys.exit(1)

    host = sys.argv[1]

    # Port range
    if len(sys.argv) >= 3 and sys.argv[2] == "top":
        ports = sorted(SERVICES.keys())
    else:
        start = int(sys.argv[2]) if len(sys.argv) > 2 else 1
        end = int(sys.argv[3]) if len(sys.argv) > 3 else 1024
        ports = list(range(start, end + 1))

    # ─── Header gaya nmap ──────────────────────────────────────
    print()
    print(f"Starting Netra scan")
    print(f"  Target : {host}")
    print(f"  Waktu  : {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print(f"  Port   : {len(ports)} port ({ports[0]}-{ports[-1]})")
    print()

    # Resolve hostname
    hostname, ip = resolve(host)
    if not ip:
        print(f"[!] Gagal resolve {host}. Cek koneksi/DNS.")
        sys.exit(1)
    print(f"Netra scan report for {hostname} ({ip})")

    # Ping / host up check + OS detect
    os_guess, latency = detect_os(ip)
    if latency is not None:
        print(f"Host is up ({latency:.3f}s latency).")
    else:
        print("Host seems up (no ping response, tapi port scan tetep jalan).")

    # ─── Scan ──────────────────────────────────────────────────
    t_start = time.time()
    print(f"Scanning {len(ports)} ports...")
    print()

    open_ports = []
    closed_count = 0

    with ThreadPoolExecutor(max_workers=100) as ex:
        results = ex.map(lambda p: scan_port(ip, p), ports)
        for port, is_open, banner, rtt in results:
            if is_open:
                open_ports.append((port, banner, rtt))
            else:
                closed_count += 1

    elapsed = time.time() - t_start
    open_ports.sort(key=lambda x: x[0])

    # ─── Output tabel ──────────────────────────────────────────
    if closed_count > 0:
        print(f"Not shown: {closed_count} closed ports")
    print()

    if not open_ports:
        print("Gak ada port terbuka ketemu.")
    else:
        # Header tabel
        print(f"{'PORT':<8} {'STATE':<8} {'SERVICE':<14} {'VERSION':<30} {'RTT'}")
        print("-" * 78)
        for port, banner, rtt in open_ports:
            svc = SERVICES.get(port, "unknown")
            ver = fmt_banner(port, banner)
            print(f"{port}/tcp  {'open':<8} {svc:<14} {ver:<30} {rtt:.0f}ms")

    # ─── OS detect summary ─────────────────────────────────────
    print()
    if os_guess:
        print(f"OS detection: {os_guess}")

    print(f"Scan selesai: {len(open_ports)} open / {len(ports)} discan "
          f"dalam {elapsed:.2f}s")
    print()


if __name__ == "__main__":
    main()
