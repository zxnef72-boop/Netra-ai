-- crypto.lua — cek harga crypto live via CoinGecko
-- Usage: /lua crypto.lua <coin>
-- Contoh: /lua crypto.lua btc
--         /lua crypto.lua eth

local coin = args[1] or "bitcoin"
coin = coin:lower()

-- Alias nama pendek
local ids = {
    btc="bitcoin", eth="ethereum", sol="solana",
    doge="dogecoin", bnb="binancecoin", ada="cardano",
    xrp="ripple", dot="polkadot", matic="matic-network",
}
if ids[coin] then coin = ids[coin] end

print("Mengambil harga " .. coin .. "...")

local url = "https://api.coingecko.com/api/v3/simple/price?ids="
         .. coin .. "&vs_currencies=usd,idr"

local body, status = http_get(url)

if status ~= 200 then
    print("Error HTTP: " .. tostring(status))
    print(body)
    return
end

-- Parse JSON sederhana
local usd = body:match('"usd":([%d%.]+)')
local idr = body:match('"idr":([%d%.]+)')

if usd and idr then
    print("")
    print("=== " .. coin:upper() .. " ===")
    print("USD : $" .. usd)
    print("IDR : Rp " .. idr)
    print("")
    print("(data dari CoinGecko, real-time)")
else
    print("Gagal parse. Response:")
    print(body)
end
