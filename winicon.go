// winicon.go — ikon EXE Windows, Go murni (tanpa ImageMagick, tanpa rsrc, tanpa NSIS).
// Alur: PNG -> ICO (PNG-in-ICO) -> resource COFF .syso yang ditulis sendiri.
// Linker Go otomatis menyertakan rsrc_windows_amd64.syso saat GOOS=windows GOARCH=amd64.
package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
)

var winIconSizes = []int{256, 128, 64, 48, 32, 16}

// generateDefaultPNG — ikon default (kotak warna dari nama + huruf inisial), 256x256.
func generateDefaultPNG(appName, outPath string) error {
	return savePNG(makeDefaultIcon(appName, 256), outPath)
}

// squareCanvas — taruh gambar di kanvas persegi tanpa merusak rasio.
func squareCanvas(src image.Image, size int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return image.NewRGBA(image.Rect(0, 0, size, size))
	}
	if w == h {
		return resizeIcon(src, size, size)
	}
	nw, nh := size, size
	if w > h {
		nh = size * h / w
	} else {
		nw = size * w / h
	}
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	scaled := resizeIcon(src, nw, nh)
	canvas := image.NewRGBA(image.Rect(0, 0, size, size))
	off := image.Pt((size-nw)/2, (size-nh)/2)
	draw.Draw(canvas, scaled.Bounds().Add(off), scaled, image.Point{}, draw.Src)
	return canvas
}

// buildICOBytes — encode ICO (setiap ukuran disimpan sebagai PNG).
func buildICOBytes(src image.Image) ([]byte, error) {
	base := squareCanvas(src, 256)
	var blobs [][]byte
	for _, s := range winIconSizes {
		var img image.Image = base
		if s != 256 {
			img = resizeIcon(base, s, s)
		}
		var pb bytes.Buffer
		if err := png.Encode(&pb, img); err != nil {
			return nil, err
		}
		blobs = append(blobs, pb.Bytes())
	}
	var out bytes.Buffer
	le := binary.LittleEndian
	binary.Write(&out, le, uint16(0))
	binary.Write(&out, le, uint16(1))
	binary.Write(&out, le, uint16(len(winIconSizes)))
	offset := 6 + 16*len(winIconSizes)
	for i, s := range winIconSizes {
		wb := byte(s)
		if s >= 256 {
			wb = 0
		}
		out.WriteByte(wb)
		out.WriteByte(wb)
		out.WriteByte(0)
		out.WriteByte(0)
		binary.Write(&out, le, uint16(1))
		binary.Write(&out, le, uint16(32))
		binary.Write(&out, le, uint32(len(blobs[i])))
		binary.Write(&out, le, uint32(offset))
		offset += len(blobs[i])
	}
	for _, b := range blobs {
		out.Write(b)
	}
	return out.Bytes(), nil
}

// prepareICO — cari ~/.netra-ai/icon.(png|jpg|jpeg), kalau tidak ada pakai ikon default.
// Hasil: <workDir>/icon.ico
func prepareICO(appName, workDir string) (string, error) {
	var src image.Image
	if home, err := os.UserHomeDir(); err == nil {
		for _, n := range []string{"icon.png", "icon.jpg", "icon.jpeg"} {
			p := filepath.Join(home, ".netra-ai", n)
			if _, err := os.Stat(p); err == nil {
				if im, err := loadIconImage(p); err == nil {
					src = im
					break
				}
			}
		}
	}
	if src == nil {
		src = makeDefaultIcon(appName, 256)
	}
	data, err := buildICOBytes(src)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return "", err
	}
	icoPath := filepath.Join(workDir, "icon.ico")
	if err := os.WriteFile(icoPath, data, 0644); err != nil {
		return "", err
	}
	return icoPath, nil
}

type icoEntry struct {
	w, h, colors uint8
	planes, bpp  uint16
	data         []byte
}

func parseICO(b []byte) ([]icoEntry, error) {
	le := binary.LittleEndian
	if len(b) < 6 || le.Uint16(b[0:]) != 0 || le.Uint16(b[2:]) != 1 {
		return nil, errors.New("bukan file ICO")
	}
	n := int(le.Uint16(b[4:]))
	if n == 0 || len(b) < 6+16*n {
		return nil, errors.New("ICO kosong atau terpotong")
	}
	var es []icoEntry
	for i := 0; i < n; i++ {
		d := b[6+16*i:]
		size := int(le.Uint32(d[8:]))
		off := int(le.Uint32(d[12:]))
		if off < 0 || size <= 0 || off+size > len(b) {
			return nil, fmt.Errorf("entri ICO %d di luar batas", i)
		}
		es = append(es, icoEntry{d[0], d[1], d[2], le.Uint16(d[4:]), le.Uint16(d[6:]), b[off : off+size]})
	}
	return es, nil
}

// buildSyso — tulis COFF AMD64 berisi section .rsrc (RT_ICON + RT_GROUP_ICON).
func buildSyso(ico []byte) ([]byte, error) {
	es, err := parseICO(ico)
	if err != nil {
		return nil, err
	}
	le := binary.LittleEndian
	n := len(es)
	const (
		dirHdr   = 16
		entSz    = 8
		dataEnt  = 16
		subdirFl = 0x80000000
		lang     = 0x0409
	)
	// tata letak: root | dirTipeIcon | dirTipeGrup | n x dirLang(icon) | dirLang(grup) | n+1 data entry | blob
	rootSz := dirHdr + 3*entSz
	iconDirSz := dirHdr + n*entSz
	grpDirSz := dirHdr + entSz
	langSz := dirHdr + entSz
	offIconDir := rootSz
	offGrpDir := offIconDir + iconDirSz
	offManDir := offGrpDir + grpDirSz
	offLang := offManDir + grpDirSz
	offData := offLang + (n+2)*langSz
	offBlob := offData + (n+2)*dataEnt

	// group icon blob
	var grp bytes.Buffer
	binary.Write(&grp, le, uint16(0))
	binary.Write(&grp, le, uint16(1))
	binary.Write(&grp, le, uint16(n))
	for i, e := range es {
		grp.WriteByte(e.w)
		grp.WriteByte(e.h)
		grp.WriteByte(e.colors)
		grp.WriteByte(0)
		binary.Write(&grp, le, e.planes)
		binary.Write(&grp, le, e.bpp)
		binary.Write(&grp, le, uint32(len(e.data)))
		binary.Write(&grp, le, uint16(i+1))
	}

	align := func(x int) int { return (x + 7) &^ 7 }
	blobOff := make([]int, n+2)
	cur := offBlob
	for i := 0; i < n; i++ {
		blobOff[i] = cur
		cur = align(cur + len(es[i].data))
	}
	blobOff[n] = cur
	cur = align(cur + grp.Len())
	blobOff[n+1] = cur
	total := align(cur + len(manifestXML))

	sec := make([]byte, total)
	put16 := func(o int, v uint16) { le.PutUint16(sec[o:], v) }
	put32 := func(o int, v uint32) { le.PutUint32(sec[o:], v) }
	dir := func(o, ids int) { put16(o+14, uint16(ids)) }

	dir(0, 3)
	put32(dirHdr+0, 3)
	put32(dirHdr+4, uint32(offIconDir)|subdirFl)
	put32(dirHdr+8, 14)
	put32(dirHdr+12, uint32(offGrpDir)|subdirFl)
	put32(dirHdr+16, 24)
	put32(dirHdr+20, uint32(offManDir)|subdirFl)

	dir(offIconDir, n)
	for i := 0; i < n; i++ {
		o := offIconDir + dirHdr + i*entSz
		put32(o, uint32(i+1))
		put32(o+4, uint32(offLang+i*langSz)|subdirFl)
	}
	dir(offGrpDir, 1)
	put32(offGrpDir+dirHdr, 1)
	put32(offGrpDir+dirHdr+4, uint32(offLang+n*langSz)|subdirFl)
	dir(offManDir, 1)
	put32(offManDir+dirHdr, 1)
	put32(offManDir+dirHdr+4, uint32(offLang+(n+1)*langSz)|subdirFl)

	var relocs []uint32
	for i := 0; i <= n+1; i++ {
		lo := offLang + i*langSz
		dir(lo, 1)
		put32(lo+dirHdr, lang)
		put32(lo+dirHdr+4, uint32(offData+i*dataEnt))
		do := offData + i*dataEnt
		var blob []byte
		if i < n {
			blob = es[i].data
		} else {
			if i == n {
				blob = grp.Bytes()
			} else {
				blob = []byte(manifestXML)
			}
		}
		put32(do, uint32(blobOff[i])) // addend; direlokasi ADDR32NB terhadap awal .rsrc
		put32(do+4, uint32(len(blob)))
		copy(sec[blobOff[i]:], blob)
		relocs = append(relocs, uint32(do))
	}

	var out bytes.Buffer
	rawPtr := 20 + 40
	relPtr := rawPtr + len(sec)
	symPtr := relPtr + 10*len(relocs)
	// file header
	binary.Write(&out, le, uint16(0x8664))
	binary.Write(&out, le, uint16(1))
	binary.Write(&out, le, uint32(0))
	binary.Write(&out, le, uint32(symPtr))
	binary.Write(&out, le, uint32(1))
	binary.Write(&out, le, uint16(0))
	binary.Write(&out, le, uint16(0))
	// section header
	out.Write([]byte{'.', 'r', 's', 'r', 'c', 0, 0, 0})
	binary.Write(&out, le, uint32(0))
	binary.Write(&out, le, uint32(0))
	binary.Write(&out, le, uint32(len(sec)))
	binary.Write(&out, le, uint32(rawPtr))
	binary.Write(&out, le, uint32(relPtr))
	binary.Write(&out, le, uint32(0))
	binary.Write(&out, le, uint16(len(relocs)))
	binary.Write(&out, le, uint16(0))
	binary.Write(&out, le, uint32(0x40000040))
	out.Write(sec)
	for _, r := range relocs {
		binary.Write(&out, le, r)
		binary.Write(&out, le, uint32(0)) // simbol 0 = section .rsrc
		binary.Write(&out, le, uint16(3)) // IMAGE_REL_AMD64_ADDR32NB
	}
	// simbol section
	out.Write([]byte{'.', 'r', 's', 'r', 'c', 0, 0, 0})
	binary.Write(&out, le, uint32(0))
	binary.Write(&out, le, uint16(1))
	binary.Write(&out, le, uint16(0))
	out.WriteByte(3)
	out.WriteByte(0)
	// string table (kosong)
	binary.Write(&out, le, uint32(4))
	return out.Bytes(), nil
}

// embedIcon — baca ICO, tulis rsrc_windows_amd64.syso di targetDir (folder paket main stub).
func embedIcon(targetDir, icoPath string) error {
	ico, err := os.ReadFile(icoPath)
	if err != nil {
		return err
	}
	syso, err := buildSyso(ico)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(targetDir, "rsrc_windows_amd64.syso"), syso, 0644)
}

// embedIconFor — siapkan ICO (dari srcPath kalau ada, kalau tidak ikon default) lalu tulis .syso ke dir.
// Kegagalan ikon tidak boleh menggagalkan build; pemanggil cukup mencatat error.
func embedIconFor(appName, dir, srcPath string) error {
	var src image.Image
	if srcPath != "" {
		if im, err := loadIconImage(srcPath); err == nil {
			src = im
		}
	}
	var icoPath string
	var err error
	if src != nil {
		var data []byte
		if data, err = buildICOBytes(src); err != nil {
			return err
		}
		icoPath = filepath.Join(dir, "icon.ico")
		if err = os.WriteFile(icoPath, data, 0644); err != nil {
			return err
		}
	} else if icoPath, err = prepareICO(appName, dir); err != nil {
		return err
	}
	return embedIcon(dir, icoPath)
}
