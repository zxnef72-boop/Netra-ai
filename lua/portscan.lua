-- portscan.lua — port scanner PARALEL via tcp_probe_many (format tabel)
-- Pakai: /lua lua/portscan.lua <host> <start> <end>

local host = args[1] or "127.0.0.1"
local start = tonumber(args[2]) or 1
local endp = tonumber(args[3]) or 1024

local SVC = {
  [21]="FTP",[22]="SSH",[23]="Telnet",[25]="SMTP",[53]="DNS",
  [80]="HTTP",[110]="POP3",[143]="IMAP",[443]="HTTPS",[445]="SMB",
  [993]="IMAPS",[995]="POP3S",[1433]="MSSQL",[3306]="MySQL",
  [3389]="RDP",[5432]="PostgreSQL",[6379]="Redis",[8080]="HTTP-alt",
  [8443]="HTTPS-alt",[27017]="MongoDB",
}

print("Scan " .. host .. " port " .. start .. "-" .. endp .. " (paralel)\n")

local ports = {}
for p = start, endp do
  table.insert(ports, p)
end

local probes = tcp_probe_many(host, ports, 1000)

local rows = {}
local found = 0
for _, pr in ipairs(probes) do
  if pr.open then
    found = found + 1
    table.insert(rows, {tostring(pr.port), "Terbuka", SVC[pr.port] or "Unknown"})
  end
end

if found == 0 then
  print("Tidak ada port terbuka pada range ini.")
else
  print_table({"Port","Status","Service"}, rows)
  print(string.format("\nTotal: %d port terbuka dari %d port yang dicek",
    found, endp - start + 1))
end
