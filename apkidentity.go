// apkidentity.go -- ganti package name + label di template APK (pure Go).
// Manifest: string pool UTF-16 (atribut package).
// resources.arsc: string pool UTF-8 (string/app_name) + nama package tabel.
package main

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"
)

const (
	templatePackage = "com.netra.template"
	templateLabel   = "NetraApp"
)

// javaKeywords -- segmen package gak boleh sama dengan keyword Java.
var javaKeywords = map[string]bool{
	"abstract": true, "assert": true, "boolean": true, "break": true, "byte": true,
	"case": true, "catch": true, "char": true, "class": true, "const": true,
	"continue": true, "default": true, "do": true, "double": true, "else": true,
	"enum": true, "extends": true, "final": true, "finally": true, "float": true,
	"for": true, "goto": true, "if": true, "implements": true, "import": true,
	"instanceof": true, "int": true, "interface": true, "long": true, "native": true,
	"new": true, "package": true, "private": true, "protected": true, "public": true,
	"return": true, "short": true, "static": true, "strictfp": true, "super": true,
	"switch": true, "synchronized": true, "this": true, "throw": true, "throws": true,
	"transient": true, "try": true, "void": true, "volatile": true, "while": true,
	"true": true, "false": true, "null": true,
}

// makePackageName -- "com.netra.<nama>" yang valid buat Android.
func makePackageName(appName string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(appName) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			sb.WriteRune(r)
		case r == '_' || r == ' ' || r == '-':
			sb.WriteRune('_')
		}
	}
	seg := strings.Trim(sb.String(), "_")
	if len(seg) > 40 {
		seg = seg[:40]
	}
	if seg == "" {
		seg = "app"
	}
	if seg[0] >= '0' && seg[0] <= '9' {
		seg = "app" + seg
	}
	if javaKeywords[seg] {
		seg = "app_" + seg
	}
	return "com.netra." + seg
}

func le16(b []byte) int { return int(binary.LittleEndian.Uint16(b)) }
func le32(b []byte) int { return int(binary.LittleEndian.Uint32(b)) }

func putLE32(b []byte, v int) { binary.LittleEndian.PutUint32(b, uint32(v)) }

func readLen8(b []byte) (int, int) {
	if len(b) < 2 {
		return 0, 1
	}
	if b[0]&0x80 != 0 {
		return int(b[0]&0x7f)<<8 | int(b[1]), 2
	}
	return int(b[0]), 1
}

func readLen16(b []byte) (int, int) {
	if len(b) < 4 {
		return le16(b), 2
	}
	v := le16(b)
	if v&0x8000 != 0 {
		return (v&0x7fff)<<16 | le16(b[2:]), 4
	}
	return v, 2
}

func readPoolString(b []byte, off int, isUTF8 bool) (string, error) {
	if off < 0 || off+2 > len(b) {
		return "", fmt.Errorf("offset string di luar chunk")
	}
	if isUTF8 {
		_, n1 := readLen8(b[off:])
		l8, n2 := readLen8(b[off+n1:])
		start := off + n1 + n2
		if start+l8 > len(b) {
			return "", fmt.Errorf("string utf8 kepanjangan")
		}
		return string(b[start : start+l8]), nil
	}
	l, n := readLen16(b[off:])
	start := off + n
	if start+l*2 > len(b) {
		return "", fmt.Errorf("string utf16 kepanjangan")
	}
	u := make([]uint16, l)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[start+i*2:])
	}
	return string(utf16.Decode(u)), nil
}

func appendLen8(b []byte, v int) []byte {
	if v > 0x7f {
		return append(b, byte(v>>8)|0x80, byte(v))
	}
	return append(b, byte(v))
}

func appendPoolString(b []byte, s string, isUTF8 bool) []byte {
	u := utf16.Encode([]rune(s))
	if isUTF8 {
		raw := []byte(s)
		b = appendLen8(b, len(u))
		b = appendLen8(b, len(raw))
		b = append(b, raw...)
		return append(b, 0)
	}
	if len(u) > 0x7fff {
		hi := len(u) >> 16
		b = append(b, byte(hi), byte(hi>>8)|0x80, byte(len(u)), byte(len(u)>>8))
	} else {
		b = append(b, byte(len(u)), byte(len(u)>>8))
	}
	for _, v := range u {
		b = append(b, byte(v), byte(v>>8))
	}
	return append(b, 0, 0)
}

// rewritePool -- bangun ulang 1 chunk string pool, tiap string lewat fn.
// Index string TIDAK berubah, jadi referensi dari chunk lain tetap valid.
func rewritePool(chunk []byte, fn func(string) string) ([]byte, error) {
	if len(chunk) < 28 || le16(chunk) != 0x0001 {
		return nil, fmt.Errorf("bukan string pool")
	}
	hdr := le16(chunk[2:])
	count := le32(chunk[8:])
	styles := le32(chunk[12:])
	flags := le32(chunk[16:])
	strStart := le32(chunk[20:])
	if styles != 0 {
		return nil, fmt.Errorf("string pool punya style, belum didukung")
	}
	if hdr+count*4 > len(chunk) {
		return nil, fmt.Errorf("tabel offset string rusak")
	}
	isUTF8 := flags&0x100 != 0

	var body []byte
	offsets := make([]int, count)
	for i := 0; i < count; i++ {
		s, err := readPoolString(chunk, strStart+le32(chunk[hdr+i*4:]), isUTF8)
		if err != nil {
			return nil, err
		}
		offsets[i] = len(body)
		body = appendPoolString(body, fn(s), isUTF8)
	}
	for len(body)%4 != 0 {
		body = append(body, 0)
	}

	newStart := hdr + count*4
	out := make([]byte, newStart, newStart+len(body))
	copy(out, chunk[:hdr])
	for i, o := range offsets {
		putLE32(out[hdr+i*4:], o)
	}
	out = append(out, body...)
	putLE32(out[4:], len(out))
	putLE32(out[20:], newStart)
	return out, nil
}

// patchAXMLPackage -- ganti atribut package di AndroidManifest.xml biner.
// Nama class activity tetap pakai package lama (cocok sama classes.dex).
func patchAXMLPackage(data []byte, oldPkg, newPkg string) ([]byte, error) {
	if len(data) < 16 || le16(data) != 0x0003 {
		return nil, fmt.Errorf("bukan AXML")
	}
	poolSize := le32(data[12:])
	if poolSize < 28 || 8+poolSize > len(data) {
		return nil, fmt.Errorf("string pool manifest rusak")
	}
	pool, err := rewritePool(data[8:8+poolSize], func(s string) string {
		if s == oldPkg {
			return newPkg
		}
		return s
	})
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(data)+len(pool))
	out = append(out, data[:8]...)
	out = append(out, pool...)
	out = append(out, data[8+poolSize:]...)
	putLE32(out[4:], len(out))
	return out, nil
}

// patchARSC -- ganti label (string/app_name) + nama package di resources.arsc.
func patchARSC(data []byte, newPkg, newLabel string) ([]byte, error) {
	if len(data) < 40 || le16(data) != 0x0002 {
		return nil, fmt.Errorf("bukan resources.arsc")
	}
	poolSize := le32(data[16:])
	if poolSize < 28 || 12+poolSize > len(data) {
		return nil, fmt.Errorf("string pool arsc rusak")
	}
	pool, err := rewritePool(data[12:12+poolSize], func(s string) string {
		if s == templateLabel {
			return newLabel
		}
		return s
	})
	if err != nil {
		return nil, err
	}
	rest := append([]byte(nil), data[12+poolSize:]...)
	if len(rest) >= 268 && le16(rest) == 0x0200 {
		u := utf16.Encode([]rune(newPkg))
		if len(u) > 127 {
			return nil, fmt.Errorf("nama package kepanjangan")
		}
		name := rest[12:268]
		for i := range name {
			name[i] = 0
		}
		for i, v := range u {
			binary.LittleEndian.PutUint16(name[i*2:], v)
		}
	}
	out := make([]byte, 0, 12+len(pool)+len(rest))
	out = append(out, data[:12]...)
	out = append(out, pool...)
	out = append(out, rest...)
	putLE32(out[4:], len(out))
	return out, nil
}

// patchTemplateIdentity -- terapin package + label baru ke entries template.
func patchTemplateIdentity(entries []APKEntry, appName string) error {
	pkg := makePackageName(appName)
	label := strings.TrimSpace(appName)
	if label == "" {
		label = templateLabel
	}
	done := 0
	for i := range entries {
		switch entries[i].Name {
		case "AndroidManifest.xml":
			d, err := patchAXMLPackage(entries[i].Data, templatePackage, pkg)
			if err != nil {
				return fmt.Errorf("manifest: %w", err)
			}
			entries[i].Data = d
			done++
		case "resources.arsc":
			d, err := patchARSC(entries[i].Data, pkg, label)
			if err != nil {
				return fmt.Errorf("resources.arsc: %w", err)
			}
			entries[i].Data = d
			done++
		}
	}
	if done != 2 {
		return fmt.Errorf("manifest/resources.arsc gak ketemu di template")
	}
	return nil
}
