// apksign.go — APK v1 (JAR) signing, pure Go stdlib.
// Gak butuh apksigner. Implementasi PKCS#7 SignedData minimal.
package main

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"
)

// ----- ASN.1 helpers -----

func asn1Len(n int) []byte {
	if n < 0x80 {
		return []byte{byte(n)}
	}
	if n < 0x100 {
		return []byte{0x81, byte(n)}
	}
	if n < 0x10000 {
		return []byte{0x82, byte(n >> 8), byte(n)}
	}
	return []byte{0x83, byte(n >> 16), byte(n >> 8), byte(n)}
}

func asn1Wrap(tag byte, data []byte) []byte {
	out := []byte{tag}
	out = append(out, asn1Len(len(data))...)
	out = append(out, data...)
	return out
}

func asn1Seq(data []byte) []byte   { return asn1Wrap(0x30, data) }
func asn1Set(data []byte) []byte   { return asn1Wrap(0x31, data) }
func asn1Octet(data []byte) []byte { return asn1Wrap(0x04, data) }
func asn1Null() []byte             { return []byte{0x05, 0x00} }
func asn1Implicit(tag byte, data []byte) []byte {
	return asn1Wrap(0xa0|tag, data)
}

func asn1BigInt(n *big.Int) []byte {
	b := n.Bytes()
	if len(b) == 0 {
		return []byte{0x02, 0x01, 0x00}
	}
	if b[0]&0x80 != 0 {
		b = append([]byte{0x00}, b...)
	}
	return asn1Wrap(0x02, b)
}

func asn1OID(oid asn1.ObjectIdentifier) []byte {
	var body []byte
	body = append(body, byte(oid[0]*40+oid[1]))
	for _, v := range oid[2:] {
		var enc []byte
		for {
			enc = append([]byte{byte(v & 0x7f)}, enc...)
			v >>= 7
			if v == 0 {
				break
			}
		}
		for i := 0; i < len(enc)-1; i++ {
			enc[i] |= 0x80
		}
		body = append(body, enc...)
	}
	return asn1Wrap(0x06, body)
}

func asn1UTCTime(t time.Time) []byte {
	s := t.UTC().Format("060102150405Z")
	return asn1Wrap(0x17, []byte(s))
}

func concat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// ----- Load key & cert -----

func loadPrivateKey(pemData []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("PEM private key kosong")
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rk, ok := k.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("bukan RSA key")
		}
		return rk, nil
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func loadCert(pemData []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("PEM cert kosong")
	}
	return x509.ParseCertificate(block.Bytes)
}

// ----- MANIFEST.MF -----

func buildManifest(entries []APKEntry) []byte {
	var sb strings.Builder
	sb.WriteString("Manifest-Version: 1.0\r\n")
	sb.WriteString("Created-By: 1.0 (Netra)\r\n")
	sb.WriteString("\r\n")
	for _, e := range entries {
		if strings.HasPrefix(e.Name, "META-INF/") {
			continue
		}
		h := sha256.Sum256(e.Data)
		d := base64.StdEncoding.EncodeToString(h[:])
		sb.WriteString("Name: " + e.Name + "\r\n")
		sb.WriteString("SHA-256-Digest: " + d + "\r\n")
		sb.WriteString("\r\n")
	}
	return []byte(sb.String())
}

func buildSF(manifestMF []byte) []byte {
	var sb strings.Builder
	sb.WriteString("Signature-Version: 1.0\r\n")
	sb.WriteString("Created-By: 1.0 (Netra)\r\n")

	h := sha256.Sum256(manifestMF)
	sb.WriteString("SHA-256-Digest-Manifest: " + base64.StdEncoding.EncodeToString(h[:]) + "\r\n")
	sb.WriteString("\r\n")

	secs := strings.Split(string(manifestMF), "\r\n\r\n")
	for _, s := range secs {
		s = strings.TrimSpace(s)
		if s == "" || strings.HasPrefix(s, "Manifest-Version:") {
			continue
		}
		d := sha256.Sum256([]byte(s + "\r\n\r\n"))
		name := ""
		for _, line := range strings.Split(s, "\r\n") {
			if strings.HasPrefix(line, "Name: ") {
				name = strings.TrimPrefix(line, "Name: ")
				break
			}
		}
		if name == "" {
			continue
		}
		sb.WriteString("Name: " + name + "\r\n")
		sb.WriteString("SHA-256-Digest: " + base64.StdEncoding.EncodeToString(d[:]) + "\r\n")
		sb.WriteString("\r\n")
	}
	return []byte(sb.String())
}

// ----- PKCS#7 -----

var (
	oidData          = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 1}
	oidSignedData    = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}
	oidSHA256        = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
	oidRSAEncryption = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}
	oidContentType   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 3}
	oidMessageDigest = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 4}
	oidSigningTime   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 5}
)

func algoIdentifier(oid asn1.ObjectIdentifier) []byte {
	return asn1Seq(concat(asn1OID(oid), asn1Null()))
}

func pkcs7Sign(content []byte, key *rsa.PrivateKey, cert *x509.Certificate) ([]byte, error) {
	contentHash := sha256.Sum256(content)

	attrContentType := asn1Seq(concat(
		asn1OID(oidContentType),
		asn1Set(asn1OID(oidData)),
	))
	attrSigningTime := asn1Seq(concat(
		asn1OID(oidSigningTime),
		asn1Set(asn1UTCTime(time.Now())),
	))
	attrMessageDigest := asn1Seq(concat(
		asn1OID(oidMessageDigest),
		asn1Set(asn1Octet(contentHash[:])),
	))

	attrList := [][]byte{attrContentType, attrSigningTime, attrMessageDigest}
	sort.Slice(attrList, func(i, j int) bool {
		return bytes.Compare(attrList[i], attrList[j]) < 0
	})
	var attrsContent []byte
	for _, a := range attrList {
		attrsContent = append(attrsContent, a...)
	}

	signedBytes := asn1Set(attrsContent)
	signedHash := sha256.Sum256(signedBytes)
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, signedHash[:])
	if err != nil {
		return nil, err
	}

	issuerAndSerial := asn1Seq(concat(cert.RawIssuer, asn1BigInt(cert.SerialNumber)))

	signerInfo := asn1Seq(concat(
		asn1BigInt(big.NewInt(1)),
		issuerAndSerial,
		algoIdentifier(oidSHA256),
		asn1Implicit(0, attrsContent),
		algoIdentifier(oidRSAEncryption),
		asn1Octet(sig),
	))

	signedData := asn1Seq(concat(
		asn1BigInt(big.NewInt(1)),
		asn1Set(algoIdentifier(oidSHA256)),
		asn1Seq(asn1OID(oidData)),
		asn1Implicit(0, cert.Raw),
		asn1Set(signerInfo),
	))

	return asn1Seq(concat(
		asn1OID(oidSignedData),
		asn1Implicit(0, signedData),
	)), nil
}

// ----- Public API -----

// signAPKv1 — tanda tangan APK bytes, return APK signed.
func signAPKv1(apkBytes []byte, key *rsa.PrivateKey, cert *x509.Certificate) ([]byte, error) {
	entries, err := extractAPK(apkBytes)
	if err != nil {
		return nil, err
	}
	entries = removeSignatureFiles(entries)

	manifestMF := buildManifest(entries)
	certSF := buildSF(manifestMF)
	certRSA, err := pkcs7Sign(certSF, key, cert)
	if err != nil {
		return nil, fmt.Errorf("sign pkcs7: %w", err)
	}

	entries = append(entries, APKEntry{Name: "META-INF/MANIFEST.MF", Data: manifestMF, Method: 8})
	entries = append(entries, APKEntry{Name: "META-INF/CERT.SF", Data: certSF, Method: 8})
	entries = append(entries, APKEntry{Name: "META-INF/CERT.RSA", Data: certRSA, Method: 8})

	return packAPK(entries)
}
