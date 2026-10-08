-- http.lua — custom HTTP request
-- Pakai: /lua lua/http.lua https://example.com

local url = args[1] or "https://example.com"
print("GET " .. url)

local status, headers, body = http_request("GET", url, {
    ["User-Agent"] = "Netra-Lua/1.0",
    ["Accept"] = "*/*"
})

print("Status: " .. status)
print("")
print("Headers:")
print(headers)
print("Body (500 char):")
print(body:sub(1, 500))
