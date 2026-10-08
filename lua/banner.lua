-- banner.lua — ambil banner service
-- Pakai: /lua lua/banner.lua 127.0.0.1 22

local host = args[1] or "127.0.0.1"
local port = tonumber(args[2]) or 22

print("Grab banner " .. host .. ":" .. port)
local banner = tcp_grab(host, port, 3000)
if banner then
    print("Banner:")
    print(banner)
else
    print("Gak ada banner / timeout")
end
