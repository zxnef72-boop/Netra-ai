-- portscan.lua — port scanner via tcp_connect
-- Pakai: /lua lua/portscan.lua 127.0.0.1 1 1024

local host = args[1] or "127.0.0.1"
local start = tonumber(args[2]) or 1
local endp = tonumber(args[3]) or 1024

print("Scan " .. host .. " port " .. start .. "-" .. endp)
print("")

local found = {}
for p = start, endp do
    if tcp_connect(host, p, 500) then
        table.insert(found, p)
        print("[+] Port " .. p .. " TERBUKA")
    end
end

print("")
print("Total: " .. #found .. " port terbuka")
