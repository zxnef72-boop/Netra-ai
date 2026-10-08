if #args < 1 then
    print("Pakai: /lua lua/fileinfo.lua <path>")
    return
end
local content, err = file_read(args[1])
if err then
    print("Error: " .. err)
    return
end
print("File : " .. args[1])
print("Size : " .. #content .. " byte")
print("Baris: " .. select(2, content:gsub("\n", "")))
print("")
print("--- Preview (200 char) ---")
print(content:sub(1, 200))
