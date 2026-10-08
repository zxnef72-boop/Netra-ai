-- detect-http.lua — deteksi HTTP server + version + info
-- Pakai: /lua lua/detect-http.lua <host> [port]
-- Contoh: /lua lua/detect-http.lua example.com
--         /lua lua/detect-http.lua 192.168.1.1 8080

local host = args[1]
if not host then
    print("Pakai: /lua lua/detect-http.lua <host> [port]")
    return
end

local port = tonumber(args[2]) or 80
print("=== HTTP Detection ===")
print("Target: " .. host .. ":" .. port)
print("")

-- 1. Connect dulu
if not tcp_connect(host, port, 3000) then
    print("[!] Port " .. port .. " TERTUTUP atau timeout")
    return
end
print("[+] Port terbuka")

-- 2. Kirim HTTP request manual
local req = "GET / HTTP/1.0\r\n" ..
            "Host: " .. host .. "\r\n" ..
            "User-Agent: Mozilla/5.0 (Netra-Lua/1.0)\r\n" ..
            "Accept: */*\r\n" ..
            "Connection: close\r\n\r\n"

local resp, err = tcp_send(host, port, req, 5000)
if not resp then
    print("[!] Gak ada response: " .. (err or "unknown"))
    return
end

print("[+] Response diterima (" .. #resp .. " byte)")
print("")

-- 3. Parse response line + headers
local status_line = resp:match("^([^\r\n]+)")
local headers_block = resp:match("\r\n(.-)\r\n\r\n") or ""

print("--- Status Line ---")
print(status_line or "(gak kebaca)")
print("")

-- 4. Extract semua headers
print("--- Headers ---")
local headers = {}
for line in headers_block:gmatch("[^\r\n]+") do
    local k, v = line:match("^([^:]+):%s*(.+)$")
    if k and v then
        headers[k:lower()] = v
        print("  " .. k .. ": " .. v)
    end
end
print("")

-- 5. Detect server + version
print("--- Server Detection ---")
local server = headers["server"] or ""
local powered = headers["x-powered-by"] or ""

if server ~= "" then
    print("  Server: " .. server)
    
    -- Cek versi umum
    if server:lower():find("nginx") then
        local v = server:match("nginx/([%d%.]+)")
        print("  >> nginx" .. (v and " v" .. v or ""))
    elseif server:lower():find("apache") then
        local v = server:match("Apache/([%d%.]+)")
        print("  >> Apache" .. (v and " v" .. v or ""))
    elseif server:lower():find("iis") then
        local v = server:match("IIS/([%d%.]+)")
        print("  >> Microsoft IIS" .. (v and " v" .. v or ""))
    elseif server:lower():find("cloudflare") then
        print("  >> Cloudflare CDN/WAF")
    elseif server:lower():find("gws") then
        print("  >> Google Web Server")
    elseif server:lower():find("litespeed") then
        print("  >> LiteSpeed")
    elseif server:lower():find("caddy") then
        print("  >> Caddy")
    else
        print("  >> Server tidak dikenali (custom?)")
    end
else
    print("  [!] Gak ada Server header")
end

if powered ~= "" then
    print("  X-Powered-By: " .. powered)
    if powered:lower():find("php") then
        local v = powered:match("PHP/([%d%.]+)")
        print("  >> PHP" .. (v and " v" .. v or ""))
    elseif powered:lower():find("express") then
        print("  >> Express.js (Node.js)")
    elseif powered:lower():find("asp%.net") then
        print("  >> ASP.NET")
    end
end
print("")

-- 6. Security headers check
print("--- Security Headers ---")
local sec = {
    "strict-transport-security",
    "content-security-policy",
    "x-frame-options",
    "x-content-type-options",
    "referrer-policy",
    "permissions-policy",
}

local score = 0
for _, h in ipairs(sec) do
    if headers[h] then
        print("  [+] " .. h)
        score = score + 1
    else
        print("  [-] " .. h .. " TIDAK ADA")
    end
end
print("")
print("Score: " .. score .. "/" .. #sec)

-- 7. Info tambahan
print("")
print("--- Info Tambahan ---")
if headers["content-type"] then
    print("  Content-Type: " .. headers["content-type"])
end
if headers["content-length"] then
    print("  Content-Length: " .. headers["content-length"])
end
if headers["set-cookie"] then
    print("  Set-Cookie: " .. headers["set-cookie"]:sub(1, 80))
end
if headers["location"] then
    print("  Location: " .. headers["location"])
end
if headers["x-generator"] then
    print("  X-Generator: " .. headers["x-generator"])
end
