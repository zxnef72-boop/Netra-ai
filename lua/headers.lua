-- headers.lua — inspect HTTP security headers
-- Pakai: /lua lua/headers.lua https://example.com

local url = args[1] or "https://example.com"
local status, headers, body = http_request("GET", url, {}, "", 10000)

print("URL: " .. url)
print("Status: " .. status)
print("")

local low = headers:lower()
local checks = {
    "strict-transport-security",
    "content-security-policy",
    "x-frame-options",
    "x-content-type-options",
    "referrer-policy",
    "permissions-policy",
}

for _, h in ipairs(checks) do
    if low:find(h) then
        print("[+] " .. h .. " ADA")
    else
        print("[-] " .. h .. " TIDAK ADA")
    end
end
