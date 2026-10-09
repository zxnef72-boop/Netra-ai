-- docker.lua — Docker-like wrapper buat proot-distro.
-- Bukan Docker beneran (Android tanpa root gak bisa).
-- Tapi syntax-nya mirip, jadi latihan pakai ini dulu.
--
-- Usage:
--   /lua docker.lua run <distro> <cmd>
--   /lua docker.lua ps
--   /lua docker.lua pull <distro>
--   /lua docker.lua rm <distro>

local sub = args[1] or "help"

local function help()
    print("== DOCKER-LIKE (proot) ==")
    print("")
    print("Subcommand:")
    print("  run <distro> <cmd>    jalanin command di distro")
    print("  ps / images           list distro terinstall")
    print("  pull <distro>         install distro baru")
    print("  rm <distro>           hapus distro")
    print("  help                  bantuan")
    print("")
    print("Contoh:")
    print("  docker.lua run alpine uname -a")
    print("  docker.lua run kali nmap -F 192.168.1.1")
    print("  docker.lua pull debian")
    print("  docker.lua ps")
    print("")
    print("Catatan: ini bukan Docker beneran, cuma wrapper proot-distro.")
end

if sub == "help" then help() return end

if sub == "ps" or sub == "images" then
    local list = proot_list()
    print("DISTRO TERINSTALL (" .. #list .. ")")
    print("---------------------")
    if #list == 0 then
        print("  (kosong)")
    else
        for _, name in ipairs(list) do
            print("  - " .. name)
        end
    end
    return
end

if sub == "run" then
    local distro = args[2]
    if not distro then print("Pakai: docker.lua run <distro> <command>") return end
    local cmdParts = {}
    for i = 3, #args do
        table.insert(cmdParts, args[i])
    end
    if #cmdParts == 0 then print("Pakai: docker.lua run <distro> <command>") return end
    local command = table.concat(cmdParts, " ")

    print("$ docker run --rm " .. distro .. " " .. command)
    print("")
    local out, code = proot_run(distro, command)
    if out == "" then out = "(tidak ada output)" end
    print(out)
    if code ~= 0 then
        print("[exit code: " .. code .. "]")
    end
    return
end

if sub == "pull" then
    local distro = args[2]
    if not distro then print("Pakai: docker.lua pull <distro>") return end
    print("Pulling image: " .. distro)
    print("(proot-distro install " .. distro .. ")")
    print("")
    local out, code = proot_install(distro)
    print(out)
    if code ~= 0 then print("[exit code: " .. code .. "]") end
    return
end

if sub == "rm" then
    local distro = args[2]
    if not distro then print("Pakai: docker.lua rm <distro>") return end
    print("Removing: " .. distro)
    local out, code = proot_remove(distro)
    print(out)
    if code ~= 0 then print("[exit code: " .. code .. "]") end
    return
end

print("Subcommand tidak dikenal: " .. sub)
help()
