-- service-detect.lua — deteksi service dari banner
-- Pakai: /lua lua/service-detect.lua 127.0.0.1 22

local host = args[1] or "127.0.0.1"
local port = tonumber(args[2]) or 22

print("Deteksi " .. host .. ":" .. port)
local banner = tcp_grab(host, port, 3000)
if not banner then
    print("Gak ada banner")
    return
end

local low = banner:lower()
print("Banner: " .. banner:sub(1, 100))
print("")

if low:find("ssh") then
    print(">> SSH server")
elseif low:find("http") then
    print(">> HTTP server")
elseif low:find("ftp") then
    print(">> FTP server")
elseif low:find("smtp") then
    print(">> SMTP server")
elseif low:find("mysql") then
    print(">> MySQL")
elseif low:find("redis") then
    print(">> Redis")
else
    print(">> Service tidak dikenali")
end
