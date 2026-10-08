// weizenbaum.go — kutipan Joseph Weizenbaum, pencipta ELIZA (1966)
package main

import "math/rand"

// weizenbaumQuotes — peringatan dari pembuat ELIZA sendiri.
// Dipasang biar siapapun yang pake vendor eliza inget:
// AI itu bukan manusia, dan "terasa ngerti" bukan "beneran ngerti".
var weizenbaumQuotes = []string{
	"\"Paparan singkat ke program komputer sederhana bisa memicu pemikiran delusi pada orang yang sepenuhnya normal.\"",
	"\"Eliza menciptakan ilusi paling luar biasa tentang 'memahami' di pikiran orang yang berbicara dengannya.\"",
	"\"Karena kita belum bisa membuat komputer bijak, jangan beri komputer tugas yang menuntut kebijaksanaan.\"",
	"\"Seberapa pun cerdas komputer, kecerdasannya selalu asing terhadap masalah manusia yang sejati.\"",
	"\"Jika kita memperlakukan satu sama lain seperti mesin, kita akan berhenti jadi manusia.\"",
	"\"Programmer merasa berkuasa karena instruksinya dipatuhi tanpa syarat. Itu berbahaya.\"",
	"\"Bayangkan penggunaan akhir dari karyamu. Jika kamu akan menekan tombol untuk membatalkannya, jangan kerjakan proyek itu.\"",
}

// weizenbaumFooter — penutup tetap (selalu muncul, gak random)
const weizenbaumFooter = "\n   Aku if-else. Aku gak ngerti. Tapi aku dengerin.\n"

// getWeizenbaumWarning — random quote + footer tetap.
func getWeizenbaumWarning() string {
	q := weizenbaumQuotes[rand.Intn(len(weizenbaumQuotes))]
	return "∴ " + q + "\n   — Joseph Weizenbaum, 1976" + weizenbaumFooter
}
