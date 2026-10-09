# Tutorial: Bikin Plugin Lua

Plugin Lua = fitur tambahan buat Netra. Gak perlu compile Go, cukup tulis file .lua.

## Fungsi yang Tersedia

| Fungsi | Kegunaan |
|--------|----------|
| print(...) | Output ke chat |
| args | Table argumen |
| http_get(url) | HTTP GET, return body + status |
| http_probe_many(urls) | HTTP paralel (max 25) |
| tcp_connect(host, port) | Cek TCP port |
| get_env(name) | Baca env var |
| file_read(path) | Baca file |
| proot_run(distro, cmd) | Jalanin di sandbox |
| proot_list() | List distro terinstall |
| sleep(ms) | Delay |

## Plugin Pertama: Hello World

Bikin file ~/netra-ai/plugins/hello.lua:

    local nama = args[1] or "dunia"
    print("Halo, " .. nama .. "!")
    print("Waktu: " .. os.date("%Y-%m-%d %H:%M:%S"))

Jalanin:

    /lua hello.lua Budi

Output:

    Halo, Budi!
    Waktu: 2026-10-09 12:00:00

## Plugin: Cek Harga Crypto

File ~/netra-ai/plugins/crypto.lua:

    local coin = args[1] or "bitcoin"
    coin = coin:lower()

    local ids = {
        btc = "bitcoin", eth = "ethereum", sol = "solana",
        doge = "dogecoin", bnb = "binancecoin"
    }
    if ids[coin] then coin = ids[coin] end

    print("Mengambil harga " .. coin .. "...")

    local url = "https://api.coingecko.com/api/v3/simple/price?ids="
             .. coin .. "&vs_currencies=usd,idr"

    local body, status = http_get(url)
    if status ~= 200 then
        print("Error HTTP: " .. status)
        return
    end

    local usd = body:match('"usd":([%d%.]+)')
    local idr = body:match('"idr":([%d%.]+)')

    if usd and idr then
        print("=== " .. coin:upper() .. " ===")
        print("USD : $" .. usd)
        print("IDR : Rp " .. idr)
    end

Jalanin:

    /lua crypto.lua btc

## Plugin: Cek Username

File ~/netra-ai/plugins/username.lua:

    local username = args[1]
    if not username then
        print("Pakai: username.lua <username>")
        return
    end

    local platforms = {
        {"GitHub", "https://github.com/"},
        {"GitLab", "https://gitlab.com/"},
        {"Reddit", "https://www.reddit.com/user/"},
    }

    local urls = {}
    for _, p in ipairs(platforms) do
        table.insert(urls, p[2] .. username)
    end

    local results = http_probe_many(urls)

    print("== USERNAME CHECK: " .. username .. " ==")
    for i, p in ipairs(platforms) do
        local st = tonumber(results[i].status) or 0
        if st == 200 then
            print("[ADA] " .. p[1] .. " - " .. urls[i])
        end
    end

Jalanin:

    /lua username.lua budi

## Plugin: Pakai Sandbox

File ~/netra-ai/plugins/scan.lua:

    local target = args[1]
    if not target then
        print("Pakai: scan.lua <host>")
        return
    end

    print("[1] Cek DNS...")
    local dns, _ = proot_run("alpine", "nslookup " .. target)
    print(dns)

    print("[2] Cek port 80...")
    local ok = tcp_connect(target, 80)
    if ok then
        print("  Port 80 TERBUKA")
    else
        print("  Port 80 tertutup")
    end

Jalanin:

    /lua scan.lua example.com

## Tips

Test plugin cepat:

    /lua nama-plugin.lua argumen

Debug dengan print:

    print("DEBUG: nilai = " .. tostring(nilai))

Handle error HTTP:

    local body, status = http_get(url)
    if status ~= 200 then
        print("Error: HTTP " .. status)
        return
    end

Parallel untuk banyak URL (jauh lebih cepat):

    local urls = {"https://a.com", "https://b.com", "https://c.com"}
    local results = http_probe_many(urls)
    for i, r in ipairs(results) do
        print(urls[i] .. " -> " .. r.status)
    end

## Direktori Plugin

Disimpen di:

    ~/netra-ai/plugins/

Atau bisa di mana aja, tinggal panggil pakai path absolute:

    /lua /path/lengkap/plugin.lua
