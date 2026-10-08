if #args < 3 then
    print("Pakai: /lua lua/calc.lua <angka1> <op> <angka2>")
    print("Op: + - * /")
    print("Contoh: /lua lua/calc.lua 15 + 27")
    return
end
local a = tonumber(args[1])
local op = args[2]
local b = tonumber(args[3])
if not a or not b then
    print("Angka gak valid")
    return
end
local hasil
if op == "+" then hasil = a + b
elseif op == "-" then hasil = a - b
elseif op == "*" then hasil = a * b
elseif op == "/" then
    if b == 0 then print("Error: bagi nol") return end
    hasil = a / b
else
    print("Operator gak dikenal: " .. op)
    return
end
print(string.format("%g %s %g = %g", a, op, b, hasil))
