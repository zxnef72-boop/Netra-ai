-- username.lua (versi tabel + URL + summary)
local username = args[1]
if not username then
    print("Cara pakai: username.lua <username>")
    return
end

local sitesPath = os.getenv("HOME") .. "/.netra-ai/sites.txt"
local content, err = file_read(sitesPath)
if err or not content then
    print("Gagal baca sites.txt: " .. tostring(err))
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
    return
end

print("Mencari username: " .. username .. "\n")

local urls = {}
for _, p in ipairs(P) do
    local ur = string.gsub(p.u, "{u}", username)
    table.insert(urls, ur)
end

local probes = http_probe_many(urls)

local results = {}
local found, notfound, error_count = 0, 0, 0

for i, p in ipairs(P) do
    local pr = probes[i]
    local st = tonumber(pr.status) or 0
    local status_text = "Error"
    
    if st == 200 or st == 301 or st == 302 then
        status_text = "Ditemukan"
        found = found + 1
    elseif st == 404 then
        status_text = "Tidak Ada"
        notfound = notfound + 1
    else
        error_count = error_count + 1
    end
    
    -- Truncate URL kalau terlalu panjang (max 40 char)
    local url_display = urls[i]
    if #url_display > 40 then
        url_display = string.sub(url_display, 1, 37) .. "..."
    end
    
    table.insert(results, {p.n, status_text, url_display})
end

-- Tabel 3 kolom: Platform, Status, URL
print_table({"Platform", "Status", "URL"}, results)

-- Summary di bawah tabel
print(string.format("\nTotal: %d | Ditemukan: %d | Tidak Ada: %d | Error: %d",
    #P, found, notfound, error_count))
