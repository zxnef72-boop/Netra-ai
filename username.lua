-- username.lua — cek username di banyak platform.
-- Sumber list: ~/.netra-ai/sites.txt (format: Nama|URL{placeholder})
-- Usage: /lua username.lua <username>

local username = args[1]
if not username or username == "" then
    print("Pakai: username.lua <username>")
    return
end
if username:sub(1,1) == "@" then username = username:sub(2) end

print("== USERNAME CHECK ==")
print("Username: " .. username)
print("")

-- Baca sites.txt
local sitesPath = os.getenv("HOME") .. "/.netra-ai/sites.txt"
local content, err = file_read(sitesPath)
if err or not content then
    print("Gagal baca sites.txt: " .. tostring(err))
    print("Buat file di: " .. sitesPath)
    return
end

local P = {}
for line in content:gmatch("[^\n]+") do
    line = string.gsub(line, "\r", "")
    if line ~= "" and line:sub(1,1) ~= "#" then
        local idx = string.find(line, "|", 1, true)
        if idx then
            local nm = string.sub(line, 1, idx - 1)
            local ur = string.sub(line, idx + 1)
            nm = nm:match("^%s*(.-)%s*$")
            ur = ur:match("^%s*(.-)%s*$")
            if nm and ur and nm ~= "" and ur ~= "" then
                table.insert(P, {n = nm, u = ur})
            end
        end
    end
end

if #P == 0 then
    print("sites.txt kosong atau format salah.")
    print("Format: Nama|https://contoh.com/{u}")
    return
end

print("Total situs: " .. #P)
print("Cek paralel... (sabar)")
print("")

-- Bangun URL
local urls = {}
for _, p in ipairs(P) do
    local ur = string.gsub(p.u, "{u}", username)
    table.insert(urls, ur)
end

local probes = http_probe_many(urls)

local NEG = {
    "not found", "doesn't exist", "does not exist",
    "page unavailable", "user not found", "no such user",
    "account suspended", "profile unavailable",
    "this account doesn't exist", "no user found",
    "tidak ditemukan", "tidak ada", "halaman tidak",
}

local function has_neg(t)
    if not t or t == "" then return false end
    local low = string.lower(t)
    for _, w in ipairs(NEG) do
        if string.find(low, w, 1, true) then return true end
    end
    return false
end

local found, notfound, unknown = {}, {}, {}

for i, p in ipairs(P) do
    local pr = probes[i]
    local st = tonumber(pr.status) or 0
    local t  = tostring(pr.title or "")
    local ogt = tostring(pr.og_title or "")
    local sz = tonumber(pr.size) or 0
    local nh = tonumber(pr.neg_hits) or 0

    if st == 404 then
        table.insert(notfound, p.n)
    elseif st == 200 or st == 301 or st == 302 then
        if has_neg(t) or has_neg(ogt) or nh >= 1 or sz < 200 then
            table.insert(notfound, p.n)
        elseif t == "" and ogt == "" then
            table.insert(unknown, { n = p.n, s = st, u = urls[i] })
        else
            table.insert(found, { n = p.n, u = urls[i] })
        end
    else
        table.insert(unknown, { n = p.n, s = st, u = urls[i] })
    end
end

table.sort(found, function(a, b) return a.n < b.n end)
table.sort(unknown, function(a, b) return a.n < b.n end)

print(string.format("Total: %d | Ditemukan: %d | Tidak ada: %d | Error: %d",
    #P, #found, #notfound, #unknown))
print("")

if #found > 0 then
    print("== DITEMUKAN (" .. #found .. ") ==")
    for _, f in ipairs(found) do
        print("[+] " .. f.n .. " -> " .. f.u)
    end
    print("")
end

if #unknown > 0 then
    print("== ERROR / BLOCK (" .. #unknown .. ") ==")
    for _, u in ipairs(unknown) do
        print("[?] " .. u.n .. " (" .. tostring(u.s) .. ") -> " .. u.u)
    end
    print("")
end

if #notfound > 0 then
    print("== TIDAK ADA (" .. #notfound .. ") ==")
    print(table.concat(notfound, ", "))
end
