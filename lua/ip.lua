-- ip.lua — IP publik (format tabel)
-- Pakai: /lua lua/ip.lua

local body, status = http_get("https://api.ipify.org")
if status == 200 then
  print_table({"Info","Value"}, {
    {"IP Publik", body},
    {"Status", tostring(status)},
    {"Endpoint", "api.ipify.org"},
  })
else
  print("Gagal ambil IP. Status: " .. status)
end
