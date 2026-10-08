-- linkfinder.lua — extract endpoint/URL dari file JS/HTML
-- Pakai: /lua lua/linkfinder.lua <file|url> [--json]
-- Contoh: /lua lua/linkfinder.lua app.js
--         /lua lua/linkfinder.lua https://example.com/script.js
--         /lua lua/linkfinder.lua https://example.com

-- ===== Setup =====
local target = args[1]
if not target then
    print("Pakai: /lua lua/linkfinder.lua <file|url>")
    print("Contoh:")
    print("  /lua lua/linkfinder.lua app.js")
    print("  /lua lua/linkfinder.lua https://example.com/app.js")
    return
end

-- ===== Load content =====
local content = nil
local source = target

if target:match("^https?://") then
    print("Fetch: " .. target)
    local status, headers, body = http_request("GET", target, {
        ["User-Agent"] = "Mozilla/5.0 (Netra-Lua/1.0)"
    }, "", 15000)
    if status == 0 or not body then
        print("[!] Gagal fetch: " .. target)
        return
    end
    content = body
    print("Status: " .. status .. " | Size: " .. #body .. " byte")
else
    local data, err = file_read(target)
    if err then
        print("[!] Gagal baca file: " .. err)
        return
    end
    content = data
    print("File: " .. target .. " | Size: " .. #data .. " byte")
end

print("")
print("=== SCAN ===")
print("")

-- ===== Extract links =====

local found = {}
local seen = {}

local function add_link(link, kind)
    if not link or link == "" then return end
    link = link:gsub("%s+$", ""):gsub("^%s+", "")
    link = link:gsub('\\/', '/')
    link = link:gsub('["\'`]$', '')
    link = link:gsub('^["\'`]', '')
    if #link < 3 or #link > 500 then return end
    if seen[link] then return end
    seen[link] = true
    table.insert(found, {url = link, kind = kind})
end

-- 1. Full URLs (http/https)
for url in content:gmatch("https?://[%w%.%-_~:/?#%[%]@!$&'()*+,;=%%]+") do
    add_link(url, "url")
end

-- 2. Absolute paths (quoted /path)
for path in content:gmatch('["\'`](/[%w%.%-_/?=&%%:{}]+)["\'`]') do
    add_link(path, "path")
end

-- 3. API endpoints common patterns
for path in content:gmatch('["\'`](/[%w%.%-_/?=&%%:{}]*/(?:api|v[0-9]+|rest|graphql|admin|user|auth|login|upload|download|config)/[%w%.%-_/?=&%%:{}]*)["\'`]') do
    add_link(path, "api")
end

-- 4. Relative URLs (unquoted fetch/axios)
for path in content:gmatch('fetch%s*%(%s*["\'`]([^"\'`]+)["\'`]') do
    add_link(path, "fetch")
end
for path in content:gmatch('["\'`]([%w%.%-_/?=&%%:{}]+%.(?:json|xml|txt|php|asp|jsp))["\'`]') do
    add_link(path, "file")
end

-- 5. Domain names
for domain in content:gmatch("(https?://[%w%.%-]+)") do
    add_link(domain, "domain")
end

-- ===== Filter noise =====
local function is_noise(link)
    -- Skip file statis
    if link:match("%.js$") and not link:match("api") and not link:match("v[0-9]") then return false end
    if link:match("%.(css|png|jpg|jpeg|gif|svg|ico|woff|woff2|ttf|map|mp4|webp)$") then
        return true
    end
    if link:match("^data:") then return true end
    if link:match("^javascript:") then return true end
    if link:match("^mailto:") then return true end
    if link:match("^#") then return true end
    if link:match("w3%.org") then return true end
    if link:match("schema%.org") then return true end
    if link:match("example%.com") then return true end
    return false
end

local clean = {}
for _, item in ipairs(found) do
    if not is_noise(item.url) then
        table.insert(clean, item)
    end
end

-- Sort
table.sort(clean, function(a, b) return a.url < b.url end)

-- ===== Output =====
if #clean == 0 then
    print("Gak ada endpoint ketemu.")
    return
end

-- Group by kind
local groups = {url = {}, path = {}, api = {}, fetch = {}, file = {}, domain = {}}
for _, item in ipairs(clean) do
    if groups[item.kind] then
        table.insert(groups[item.kind], item.url)
    end
end

local order = {"api", "url", "path", "fetch", "file", "domain"}
local labels = {
    api = "API ENDPOINTS",
    url = "FULL URLS",
    path = "ABSOLUTE PATHS",
    fetch = "FETCH/AXIOS CALLS",
    file = "STATIC FILES",
    domain = "DOMAINS",
}

for _, kind in ipairs(order) do
    local items = groups[kind]
    if #items > 0 then
        print("--- " .. labels[kind] .. " (" .. #items .. ") ---")
        local max = 30
        for i, url in ipairs(items) do
            if i > max then
                print("  ... (" .. (#items - max) .. " lainnya)")
                break
            end
            print("  " .. url)
        end
        print("")
    end
end

print("=== TOTAL: " .. #clean .. " link ===")
