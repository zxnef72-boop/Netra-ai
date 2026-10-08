-- username.lua — cek username di banyak platform (parallel + body check).
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

local P = {
    { "GitHub",         "https://github.com/" },
    { "GitLab",         "https://gitlab.com/" },
    { "Bitbucket",      "https://bitbucket.org/" },
    { "Instagram",      "https://www.instagram.com/" },
    { "Twitter/X",      "https://x.com/" },
    { "TikTok",         "https://www.tiktok.com/@" },
    { "Reddit",         "https://www.reddit.com/user/" },
    { "Pinterest",      "https://www.pinterest.com/" },
    { "LinkedIn",       "https://www.linkedin.com/in/" },
    { "Telegram",       "https://t.me/" },
    { "Medium",         "https://medium.com/@" },
    { "Dev.to",         "https://dev.to/" },
    { "Hashnode",       "https://hashnode.com/@" },
    { "YouTube",        "https://www.youtube.com/@" },
    { "Vimeo",          "https://vimeo.com/" },
    { "Twitch",         "https://www.twitch.tv/" },
    { "Mastodon",       "https://mastodon.social/@" },
    { "Threads",        "https://www.threads.net/@" },
    { "Bluesky",        "https://bsky.app/profile/" },
    { "Snapchat",       "https://www.snapchat.com/add/" },
    { "Facebook",       "https://www.facebook.com/" },
    { "Behance",        "https://www.behance.net/" },
    { "Dribbble",       "https://dribbble.com/" },
    { "DeviantArt",     "https://www.deviantart.com/" },
    { "ArtStation",     "https://www.artstation.com/" },
    { "Flickr",         "https://www.flickr.com/people/" },
    { "Imgur",          "https://imgur.com/user/" },
    { "500px",          "https://500px.com/p/" },
    { "Unsplash",       "https://unsplash.com/@" },
    { "SoundCloud",     "https://soundcloud.com/" },
    { "Bandcamp",       "https://bandcamp.com/" },
    { "Spotify",        "https://open.spotify.com/user/" },
    { "Last.fm",        "https://www.last.fm/user/" },
    { "Stack Overflow", "https://stackoverflow.com/users/" },
    { "HackerNews",     "https://news.ycombinator.com/user?id=" },
    { "Replit",         "https://replit.com/@" },
    { "CodePen",        "https://codepen.io/" },
    { "Codewars",       "https://www.codewars.com/users/" },
    { "LeetCode",       "https://leetcode.com/" },
    { "Codeforces",     "https://codeforces.com/profile/" },
    { "HackerOne",      "https://hackerone.com/" },
    { "Bugcrowd",       "https://bugcrowd.com/" },
    { "TryHackMe",      "https://tryhackme.com/p/" },
    { "HackTheBox",     "https://app.hackthebox.com/profile/" },
    { "Keybase",        "https://keybase.io/" },
    { "Kaggle",         "https://www.kaggle.com/" },
    { "Docker Hub",     "https://hub.docker.com/u/" },
    { "NPM",            "https://www.npmjs.com/~" },
    { "PyPI",           "https://pypi.org/user/" },
    { "Gitea",          "https://gitea.com/" },
    { "SourceForge",    "https://sourceforge.net/u/" },
    { "Steam",          "https://steamcommunity.com/id/" },
    { "Xbox",           "https://xboxgamertag.com/search/" },
    { "Itch.io",        "https://itch.io/profile/" },
    { "NameMC",         "https://namemc.com/profile/" },
    { "Speedrun",       "https://www.speedrun.com/user/" },
    { "Roblox",         "https://www.roblox.com/user.aspx?username=" },
    { "Chess.com",      "https://www.chess.com/member/" },
    { "Lichess",        "https://lichess.org/@" },
    { "WordPress",      "https://" .. username .. ".wordpress.com" },
    { "Blogger",        "https://" .. username .. ".blogspot.com" },
    { "Substack",       "https://substack.com/@" },
    { "Quora",          "https://www.quora.com/profile/" },
    { "Wattpad",        "https://www.wattpad.com/user/" },
    { "AO3",            "https://archiveofourown.org/users/" },
    { "Goodreads",      "https://www.goodreads.com/" },
    { "Patreon",        "https://www.patreon.com/" },
    { "Ko-fi",          "https://ko-fi.com/" },
    { "BuyMeACoffee",   "https://www.buymeacoffee.com/" },
    { "Gumroad",        "https://gumroad.com/" },
    { "Etsy",           "https://www.etsy.com/shop/" },
    { "Redbubble",      "https://www.redbubble.com/people/" },
    { "AngelList",      "https://angel.co/u/" },
    { "About.me",       "https://about.me/" },
    { "ProductHunt",    "https://www.producthunt.com/@" },
    { "Linktree",       "https://linktr.ee/" },
    { "Duolingo",       "https://www.duolingo.com/profile/" },
    { "Strava",         "https://www.strava.com/athletes/" },
    { "VK",             "https://vk.com/" },
    { "Letterboxd",     "https://letterboxd.com/" },
    { "Trakt",          "https://trakt.tv/users/" },
    { "MyAnimeList",    "https://myanimelist.net/profile/" },
    { "AniList",        "https://anilist.co/user/" },
    { "Backloggd",      "https://backloggd.com/u/" },
}

local total = #P
local urls = {}
for _, p in ipairs(P) do
    table.insert(urls, p[2] .. username)
end

local probes = http_probe_many(urls)

-- Kata negatif (kalau ada di title -> user gak ada)
local NEG = {
    "not found", "notfound", "doesn't exist", "does not exist",
    "page not found", "user not found", "profile not found",
    "no user", "no such", "sorry, this page", "page unavailable",
    "tidak ditemukan", "tidak ada", "404", "error 404",
    "account suspended", "account not found", "profile unavailable",
    "tidak tersedia", "halaman tidak ditemukan",
}

local function has_neg(t)
    if not t or t == "" then return false end
    local low = string.lower(t)
    for _, w in ipairs(NEG) do
        if string.find(low, w, 1, true) then return true end
    end
    return false
end

local found = {}
local notfound = {}
local unknown = {}

for i, p in ipairs(P) do
    local pr = probes[i]
    local st = tonumber(pr.status) or 0
    local t  = tostring(pr.title or "")
    local sz = tonumber(pr.size) or 0

    if st == 404 then
        table.insert(notfound, p[1])
    elseif st == 200 or st == 301 or st == 302 then
        if has_neg(t) then
            table.insert(notfound, p[1])
        elseif sz < 200 then
            -- body terlalu pendek, kemungkinan not found
            table.insert(notfound, p[1])
        else
            table.insert(found, { n = p[1], u = urls[i], t = t })
        end
    else
        table.insert(unknown, { n = p[1], s = st, u = urls[i] })
    end
end

print(string.format("Total platform: %d", total))
print(string.format("Ditemukan: %d | Tidak ada: %d | Error: %d", #found, #notfound, #unknown))
print("")

local function pad(s, n)
    if #s >= n then return s:sub(1, n) end
    return s .. string.rep(" ", n - #s)
end

if #found > 0 then
    print(pad("PLATFORM", 16) .. " | " .. pad("STATUS", 6) .. " | URL")
    print(string.rep("-", 16) .. "-+-" .. string.rep("-", 6) .. "-+-" .. string.rep("-", 40))
    for _, f in ipairs(found) do
        print(pad(f.n, 16) .. " | " .. pad("200", 6) .. " | " .. f.u)
    end
    print("")
end

if #unknown > 0 then
    print("== ERROR / BLOCK ==")
    for _, u in ipairs(unknown) do
        print(pad(u.n, 16) .. " | " .. pad(tostring(u.s), 6) .. " | " .. u.u)
    end
    print("")
end

if #notfound > 0 then
    local line = ""
    for _, n in ipairs(notfound) do
        line = line .. n .. "  "
    end
    print("== TIDAK ADA (" .. #notfound .. ") ==")
    print(line)
end
