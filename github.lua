-- github.lua — info profil GitHub user
-- Usage: /lua github.lua <username>

local user = args[1]
if not user then print("Pakai: github.lua <username>") return end

print("Ambil data: " .. user .. "...")

local url = "https://api.github.com/users/" .. user
local body, status = http_get(url)

if status ~= 200 then
    print("Error HTTP: " .. tostring(status))
    print(body)
    return
end

local name  = body:match('"name":%s*"([^"]*)"') or "-"
local bio   = body:match('"bio":%s*"([^"]*)"') or "-"
local repos = body:match('"public_repos":%s*(%d+)') or "0"
local foll  = body:match('"followers":%s*(%d+)') or "0"
local loc   = body:match('"location":%s*"([^"]*)"') or "-"
local login = body:match('"login":%s*"([^"]*)"') or user

print("")
print("=== " .. login:upper() .. " ===")
print("Nama    : " .. name)
print("Bio     : " .. bio)
print("Repo    : " .. repos)
print("Follower: " .. foll)
print("Lokasi  : " .. loc)
