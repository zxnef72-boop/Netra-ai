// apktemplate.go — embed template APK + signing material ke binary Netra.
package main

import _ "embed"

//go:embed base.apk
var baseAPKBytes []byte

//go:embed debug-key.pem
var debugKeyPEM []byte

//go:embed debug-cert.pem
var debugCertPEM []byte
