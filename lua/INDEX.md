# Index File Lua Netra

## Yang sering dipakai
- portscan.lua — scan port
- banner.lua — grab banner
- detect-http.lua — deteksi service HTTP

## Yang internal (dipanggil dari Go)
- http.lua — helper HTTP request
- fetch.lua — fetch URL

## Yang mungkin jarang
- calc.lua, hitung.lua — kalkulator (sample?)
- fileinfo.lua, sysinfo.lua — info sistem
- report.lua, service-detect.lua — reporting
- linkfinder.lua — cari link (paling gede, 4.5KB)

## Orphan (perlu dicek)
- osint-templates/*.py — 4 script Python, gak dipanggil Go
