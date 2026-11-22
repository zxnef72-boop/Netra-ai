-- z_brain.lua -- otak chatbot offline Netra (tanpa AI online).
-- Jalan otomatis di mode Eliza: tiap pesan user lewat sini dulu.
-- Cara kerja: tiap intent punya pola + bobot, intent dengan skor tertinggi menang.
-- Ingatan (nama, topik, intent terakhir) disimpan lewat state_get/state_set.
-- Edit tabel INTENTS di bawah untuk ngubah karakter bot, lalu /lua-rules-reload.

-- ===== util =====
local text = " " .. (input_lower or ""):gsub("[%p]", " "):gsub("%s+", " ") .. " "

local function has(word)
  return text:find(" " .. word .. " ", 1, true) ~= nil
end

local function has_any(list)
  for _, w in ipairs(list) do
    if w:find(" ", 1, true) then
      if text:find(w, 1, true) then return true end
    elseif has(w) then
      return true
    end
  end
  return false
end

-- pilih jawaban acak yang beda dari yang terakhir dipakai
local function pick(key, list)
  local last = tonumber(state_get("last_" .. key)) or 0
  local n = #list
  if n == 1 then return list[1] end
  local i = math.random(n)
  if i == last then i = (i % n) + 1 end
  state_set("last_" .. key, tostring(i))
  return list[i]
end

-- ganti sudut pandang: aku -> kamu, dst
local function reflect(s)
  local map = {
    aku = "kamu", saya = "kamu", gue = "kamu", gw = "kamu",
    punyaku = "punyamu", milikku = "milikmu", ku = "mu",
    kamu = "aku", anda = "aku",
  }
  local out = {}
  for w in s:gmatch("%S+") do
    out[#out + 1] = map[w:lower()] or w
  end
  return table.concat(out, " ")
end

-- ===== kapan brain mundur (biar fitur lain tetap jalan) =====
local SKIP_PREFIX = {
  "cara ", "apa itu ", "baca ", "cari ", "html ", "scan ", "fetch ", "buka ",
  "github ", "crypto ", "hitung ", "convert ", "tutorial ", "template ",
  "/", "http",
}

local function should_skip()
  local raw = input_lower or ""
  if raw:match("^%s*$") then return true end
  if raw:find("http", 1, true) or raw:find("www.", 1, true) then return true end
  for _, p in ipairs(SKIP_PREFIX) do
    if raw:sub(1, #p) == p then return true end
  end
  return false
end

-- ===== intent =====
-- pola: kata utuh (cocok per kata) atau frasa (ada spasi). bobot makin besar makin kuat.
local INTENTS = {
  salam = {
    words = { "halo", "hai", "hi", "hello", "hey", "pagi", "siang", "sore", "malam", "assalamualaikum", "permisi" },
    weight = 2, only_short = true,
  },
  kabar = {
    words = { "apa kabar", "gimana kabar", "kabar kamu", "lagi apa", "sedang apa" },
    weight = 3,
  },
  pamit = {
    words = { "dadah", "bye", "sampai jumpa", "pamit", "aku pergi dulu", "udahan", "sudah dulu", "duluan" },
    weight = 3,
  },
  terima_kasih = {
    words = { "makasih", "terima kasih", "thanks", "thx", "tengkyu" },
    weight = 3,
  },
  siapa_kamu = {
    words = { "kamu siapa", "siapa kamu", "namamu siapa", "nama kamu siapa", "kamu itu apa", "kamu bot" },
    weight = 4,
  },
  bisa_apa = {
    words = { "kamu bisa apa", "bisa apa", "fitur", "apa yang bisa", "bantu apa", "bisa bantu" },
    weight = 3,
  },
  minta_bantuan = {
    words = { "bantu", "tolong", "bantuin", "help", "bantuan" },
    weight = 2,
  },
  coding = {
    words = { "error", "bug", "kode", "coding", "program", "script", "golang", "python", "lua", "javascript",
              "html", "css", "compile", "build", "crash", "git", "termux", "apk", "fungsi", "variabel", "syntax" },
    weight = 2,
  },
  sedih = {
    words = { "sedih", "galau", "kecewa", "nangis", "sendirian", "kesepian", "putus asa", "down", "hancur", "sakit hati" },
    weight = 3,
  },
  capek = {
    words = { "capek", "lelah", "cape", "ngantuk", "burnout", "penat", "jenuh", "bosan", "bosen", "stres", "stress", "pusing" },
    weight = 3,
  },
  senang = {
    words = { "senang", "seneng", "bahagia", "asik", "mantap", "berhasil", "lancar", "sukses", "yeay" },
    weight = 2,
  },
  marah = {
    words = { "marah", "kesal", "kesel", "emosi", "jengkel", "benci", "sebel" },
    weight = 3,
  },
  takut = {
    words = { "takut", "cemas", "khawatir", "gelisah", "panik", "was was" },
    weight = 3,
  },
  waktu = {
    words = { "jam berapa", "tanggal berapa", "hari apa", "sekarang jam" },
    weight = 4,
  },
  setuju = {
    words = { "iya", "ya", "yup", "betul", "benar", "oke", "ok", "siap", "boleh", "mau" },
    weight = 1, only_short = true,
  },
  tidak = {
    words = { "tidak", "nggak", "gak", "ga", "enggak", "belum", "bukan", "jangan" },
    weight = 1, only_short = true,
  },
}

local function wordcount()
  local n = 0
  for _ in text:gmatch("%S+") do n = n + 1 end
  return n
end

local function best_intent()
  local n = wordcount()
  local best, best_score = nil, 0
  for name, it in pairs(INTENTS) do
    local score = 0
    if not (it.only_short and n > 3) then
      for _, w in ipairs(it.words) do
        if has_any({ w }) then score = score + it.weight end
      end
    end
    if score > best_score or (score == best_score and score > 0 and name < (best or "~")) then
      best, best_score = name, score
    end
  end
  return best, best_score
end

-- ===== jawaban =====
local R = {}

R.salam = function(c)
  local nama = state_get("nama")
  local g = (c and c.greeting) or "hari"
  local sapa = nama ~= "" and (" " .. nama) or ""
  return pick("salam", {
    "Halo" .. sapa .. ". Ada yang bisa dibantu?",
    "Selamat " .. g .. sapa .. ". Lagi ngerjain apa?",
    "Hai" .. sapa .. ". Mau ngobrol atau butuh bantuan?",
  })
end

R.kabar = function()
  return pick("kabar", {
    "Baik, terima kasih. Kamu sendiri gimana?",
    "Aman. Hari ini lagi ngerjain apa?",
  })
end

R.pamit = function()
  state_set("topic", "")
  return pick("pamit", { "Sampai jumpa. Lanjut kapan saja.", "Oke, hati-hati. Kabari lagi kalau butuh.", "Siap. Sampai nanti." })
end

R.terima_kasih = function()
  return pick("tk", { "Sama-sama.", "Sama-sama, senang bisa bantu.", "Siap. Ada lagi yang perlu dibantu?" })
end

R.siapa_kamu = function()
  return "Aku chatbot offline Netra. Jawabanku dari aturan yang ditulis di Lua, bukan dari AI online, jadi terbatas tapi cepat dan jalan tanpa internet."
end

R.bisa_apa = function()
  return "Aku bisa ngobrol santai, bantu cari arah waktu ada masalah coding (error, build, Termux), dan nemenin kalau kamu lagi capek atau stres. Untuk tutorial dan pencarian, tulis misalnya: cara bikin apk, atau apa itu golang."
end

R.minta_bantuan = function()
  state_set("topic", "bantuan")
  return pick("bantu", {
    "Bisa. Ceritain masalahnya: apa yang mau dikerjain dan di mana mentoknya?",
    "Boleh. Jelasin dulu situasinya, lengkap sama pesan error kalau ada.",
  })
end

R.coding = function()
  state_set("topic", "coding")
  local raw = input_raw or ""
  local err = raw:match("[Ee]rror[:%s]+(.+)")
  if err then
    return "Soal error itu: coba baca baris pertama pesannya, biasanya menunjuk file dan nomor baris. Error-nya muncul di langkah mana, saat build atau saat dijalankan?"
  end
  return pick("coding", {
    "Soal coding ya. Pakai bahasa apa, dan bagian mana yang bikin mentok?",
    "Oke. Tempel potongan kode atau pesan error-nya, biar lebih jelas.",
    "Kamu lagi ngerjain apa persisnya, dan hasil yang diharapkan seperti apa?",
  })
end

R.sedih = function()
  state_set("topic", "perasaan")
  return pick("sedih", {
    "Kedengarannya berat. Mau ceritain apa yang bikin kamu merasa begitu?",
    "Aku di sini. Apa yang paling bikin kepikiran?",
  })
end

R.capek = function()
  state_set("topic", "perasaan")
  return pick("capek", {
    "Wajar kalau capek. Udah sempat istirahat hari ini?",
    "Kedengarannya kamu perlu jeda. Apa yang paling menguras tenaga belakangan ini?",
  })
end

R.senang = function()
  return pick("senang", { "Bagus. Apa yang bikin kamu senang?", "Ikut senang dengarnya. Ceritain dong." })
end

R.marah = function()
  state_set("topic", "perasaan")
  return pick("marah", {
    "Kedengarannya kamu kesal banget. Apa yang terjadi?",
    "Boleh cerita. Apa yang bikin kamu marah?",
  })
end

R.takut = function()
  state_set("topic", "perasaan")
  return pick("takut", {
    "Apa yang paling bikin kamu cemas soal itu?",
    "Pelan-pelan aja. Apa yang kamu takutkan akan terjadi?",
  })
end

R.waktu = function()
  return "Sekarang " .. (time_hm() or "") .. ", tanggal " .. (date_today() or "") .. "."
end

R.setuju = function()
  local t = state_get("topic")
  if t == "coding" or t == "bantuan" then
    return "Oke. Lanjut, kasih detailnya: kode atau pesan error yang dihadapi?"
  elseif t == "perasaan" then
    return "Aku dengerin. Lanjutkan saja ceritanya."
  end
  return pick("setuju", { "Oke. Lanjut?", "Siap. Ada yang mau dibahas?" })
end

R.tidak = function()
  local t = state_get("topic")
  if t == "perasaan" then
    return "Nggak apa-apa. Kalau belum mau cerita, kita bahas hal lain dulu."
  end
  return pick("tidak", { "Oke, nggak masalah.", "Baik. Ada hal lain yang mau dibahas?" })
end

-- ===== ekstraksi (nama, kalimat perasaan) =====
local function try_name()
  local raw = input_raw or ""
  local n = raw:match("[Nn]ama%s+[Aa]ku%s+(%a+)")
    or raw:match("[Nn]amaku%s+(%a+)")
    or raw:match("[Pp]anggil%s+[Aa]ku%s+(%a+)")
  if n then return n:sub(1, 1):upper() .. n:sub(2):lower() end
  return nil
end

local function try_reflect()
  local raw = input_raw or ""
  local s = raw:match("[Aa]ku%s+merasa%s+(.+)")
    or raw:match("[Aa]ku%s+lagi%s+(.+)")
    or raw:match("[Aa]ku%s+sedang%s+(.+)")
    or raw:match("[Aa]ku%s+nggak%s+bisa%s+(.+)")
  if s and #s > 2 then
    s = reflect(s:gsub("[%.%!%?]+%s*$", ""))
    return s
  end
  return nil
end

-- ===== entry point =====
function match(inp)
  if should_skip() then return false end
  return true
end

function reply(inp, c)
  -- 1. perkenalan nama
  local nm = try_name()
  if nm then
    state_set("nama", nm)
    return "Salam kenal, " .. nm .. ". Ada yang bisa dibantu?"
  end

  -- 2. intent dengan skor tertinggi
  local intent, score = best_intent()

  -- 3. pantulan ala Eliza ("aku merasa ...", "aku lagi ..."), diutamakan
  --    kalau intent yang ketemu lemah (skor < 2)
  if not intent or score < 2 then
    local r = try_reflect()
    if r then
      state_set("topic", "perasaan")
      return pick("refl", {
        "Kenapa kamu " .. r .. "?",
        "Sejak kapan kamu " .. r .. "?",
        "Apa yang bikin kamu " .. r .. "?",
      })
    end
  end

  if intent and score >= 1 then
    state_set("last_intent", intent)
    return R[intent](c)
  end

  -- 4. fallback: nyambung ke topik terakhir, bukan jawaban acak
  local t = state_get("topic")
  if t == "coding" then
    return "Boleh dijelasin lebih detail? Bagian kode atau error yang mana?"
  elseif t == "perasaan" then
    return "Lanjutkan saja, aku dengerin."
  end
  return pick("fallback", {
    "Aku belum paham maksudnya. Bisa diulang dengan kata lain?",
    "Kurang jelas buatku. Ceritain lebih lengkap?",
    "Soal itu aku belum punya jawabannya. Mau bahas topik lain, atau jelasin lagi?",
  })
end
