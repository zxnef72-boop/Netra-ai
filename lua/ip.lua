local body, status = http_get("https://api.ipify.org")
if status == 200 then
    print("IP publik: " .. body)
else
    print("Gagal. Status: " .. status)
end
