-- service-detect.lua — deteksi service dari banner (format tabel)
-- Pakai: /lua lua/service-detect.lua <host> <port>

local host = args[1] or "127.0.0.1"
local port = tonumber(args[2]) or 22

local banner = tcp_grab(host, port, 3000)
if not banner then
  print("Tidak ada banner pada " .. host .. ":" .. port)
  return
end

local low = banner:lower()
local svc = "Tidak dikenali"
if low:find("ssh",1,true) then svc="SSH server"
elseif low:find("http",1,true) then svc="HTTP server"
elseif low:find("ftp",1,true) then svc="FTP server"
elseif low:find("smtp",1,true) then svc="SMTP server"
elseif low:find("mysql",1,true) then svc="MySQL"
elseif low:find("redis",1,true) then svc="Redis" end

local bn = banner:gsub("[\r\n]"," ")
if #bn > 60 then bn = string.sub(bn,1,57) .. "..." end

print_table({"Host","Port","Service","Banner"}, {{host, tostring(port), svc, bn}})
