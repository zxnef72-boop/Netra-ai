if #args < 1 then
    print("Pakai: /lua lua/fetch.lua <url>")
    return
end
local url = args[1]
print("Fetch: " .. url)
local body, status = http_get(url)
if status == 0 then
    print("✗ Gagal konek")
else
    print("Status: " .. status)
    print("Size: " .. #body .. " byte")
    print("Preview:")
    print(body:sub(1, 300))
end
