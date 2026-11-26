-- headers.lua — inspect HTTP security headers (format tabel)
-- Pakai: /lua lua/headers.lua <url>

local url = args[1] or "https://example.com"
local status, headers, body = http_request("GET", url, {}, "", 10000)

print("URL: " .. url)
print("Status: " .. status .. "\n")

local low = headers:lower()
local checks = {
  "strict-transport-security","content-security-policy","x-frame-options",
  "x-content-type-options","referrer-policy","permissions-policy",
}

local rows = {}
local ada = 0
for _, h in ipairs(checks) do
  if low:find(h, 1, true) then
    ada = ada + 1
    table.insert(rows, {h, "ADA"})
  else
    table.insert(rows, {h, "TIDAK ADA"})
  end
end

print_table({"Header","Status"}, rows)
print(string.format("\nTotal: %d header | ADA: %d | TIDAK ADA: %d",
  #checks, ada, #checks - ada))
