// about_eliza.go — /about-eliza
// Kenapa ELIZA ada di netra-ai, siapa Weizenbaum, dan kenapa ini penting.
package main

func (m *Model) handleAboutEliza() {
	text := `# About ELIZA

## Apa ini?

ELIZA adalah chatbot pertama di dunia. Dibikin tahun 1966 oleh 
Joseph Weizenbaum di MIT. Programnya cuma if-else + regex — 
gak ada AI, gak ada neural network, gak ada pembelajaran.

Tapi orang beneran curhat ke dia.

---

## Kenapa disebut "ELIZA"?

Weizenbaum ngambil nama dari Eliza Doolittle — tokoh di drama 
Pygmalion (1913) karya George Bernard Shaw. Di drama itu, Eliza 
adalah gadis jalanan yang diajarin cara ngomong sopan.

Sama kayak chatbot-nya: "diajarin" pola ngobrol, tapi gak pernah 
beneran ngerti.

---

## Kenapa dulu orang percaya?

Bukan karena pintar. Karena Weizenbaum pake trik:

1. Mirroring — user bilang "saya sedih", Eliza jawab "kenapa kamu sedih?"
2. Fallback cerdas — kalo gak ngerti, dia jawab "ceritakan lebih lanjut"
3. Role play — dia pura-pura jadi psikolog (Rogerian therapist)

Otak manusia otomatis ngisi kekosongan. Kalo ada yang dengerin 
dan nanya balik, kita asumsi dia "ngerti".

Fenomena ini sekarang disebut "ELIZA Effect".

---

## Reaksi Weizenbaum

Dia syok. Sekretarisnya minta waktu privat buat "ngobrol" sama 
ELIZA. Orang-orang yang tau itu cuma program tetep curhat hal 
pribadi.

Tahun 1976, dia nulis buku:

  "Computer Power and Human Reason: From Judgment to Calculation"

Isi utamanya: AI berbahaya kalo disalahgunain.

Dia bilang:
- Komputer gak boleh dikasih tugas yang butuh kebijaksanaan
- Psikoterapi, hakim, guru, dokter — jangan digantiin AI
- Manusia punya tanggung jawab moral, komputer engga

---

## Kutipan Weizenbaum

> "What I had not realized is that extremely short exposures to a 
> relatively simple computer program could induce powerful delusional 
> thinking in quite normal people."

Terjemahan: Paparan singkat ke program komputer sederhana bisa bikin 
orang normal percaya hal yang salah.

---

> "However much intelligence computers may attain, now or in the future, 
> theirs must always be an intelligence alien to genuine human problems 
> and concerns."

Terjemahan: Seberapa pun cerdas komputer, kecerdasannya selalu asing 
terhadap masalah manusia yang sejati.

---

> "If we treat each other like machines, if we act like machines, then 
> we will become no longer human."

Terjemahan: Kalo kita perlakukan sesama seperti mesin, kita berhenti 
jadi manusia.

---

## Kenapa ELIZA ada di netra-ai?

Bukan buat nostalgia doang.

Ini monumen kecil. Pengingat dari mana AI berasal.

- ChatGPT = AI yang berusaha jadi manusia
- ELIZA = AI yang ngaku AI

Kontras ini penting. Karena:

- AI modern jauh lebih canggih, tapi tetep bukan manusia
- AI bisa niru empati, tapi gak bisa ngerasain sakit
- AI bisa ngalahin manusia di catur, tapi gak punya masa depan

---

## Yang perlu diingat

1. "Terasa manusiawi" BUKAN "beneran manusia"
2. "Pinter jawab" BUKAN "beneran ngerti"
3. "Kelihatan peduli" BUKAN "beneran peduli"

Eliza jujur: dia if-else. Dia gak nyamar jadi pinter.

AI modern kadang lupa itu.

---

  "Aku if-else. Aku gak ngerti. Tapi aku dengerin."
  — Netra AI, 2026

---

Ketik /vendor 5 buat ngobrol sama Eliza.
Ketik /help buat command lain.`

	m.messages = append(m.messages, ChatMsg{Role: "assistant", Text: text})
}
