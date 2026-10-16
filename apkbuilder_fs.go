// apkbuilder_fs.go — versi fullscreen dari APK builder.
// Pakai template ~/mainactivity_fullscreen.txt (tanpa toolbar/splash).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// writeJavaSourcesPureFullscreen — tulis hanya MainActivity, tanpa Splash.
func writeJavaSourcesPureFullscreen(cfg apkConfig) error {
	pkgPath := strings.ReplaceAll(cfg.Package, ".", "/")
	srcDir := filepath.Join(cfg.WorkDir, "src", pkgPath)

	home, _ := os.UserHomeDir()
	templatePath := filepath.Join(home, "mainactivity_fullscreen.txt")
	templateBytes, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("template fullscreen gak ketemu: %w", err)
	}

	template := string(templateBytes)
	template = strings.ReplaceAll(template, "__TARGETURL__", cfg.TargetURL)
	template = strings.ReplaceAll(template, "__APPNAME__", cfg.AppName)

	mainSrc := fmt.Sprintf(template, cfg.Package)
	return os.WriteFile(filepath.Join(srcDir, "MainActivity.java"), []byte(mainSrc), 0644)
}

// writeManifestFullscreen — manifest dengan MainActivity sebagai LAUNCHER, tanpa Splash.
func writeManifestFullscreen(cfg apkConfig) error {
	manifest := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="%s"
    android:versionCode="%d"
    android:versionName="%s">

    <uses-sdk android:minSdkVersion="%d" android:targetSdkVersion="%d" />
    <uses-permission android:name="android.permission.INTERNET" />
    <uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />
    <uses-permission android:name="android.permission.WAKE_LOCK" />
    <uses-permission android:name="android.permission.WRITE_EXTERNAL_STORAGE" android:maxSdkVersion="28" />
    <uses-permission android:name="android.permission.READ_EXTERNAL_STORAGE" />
    <uses-permission android:name="android.permission.DOWNLOAD_WITHOUT_NOTIFICATION" />

    <application
        android:label="%s"
        android:icon="@mipmap/ic_launcher"
        android:usesCleartextTraffic="true"
        android:networkSecurityConfig="@xml/network_security_config"
        android:debuggable="false"
        android:allowBackup="false"
        android:supportsRtl="true">
        <activity
            android:name=".MainActivity"
            android:theme="@android:style/Theme.NoTitleBar"
            android:configChanges="orientation|screenSize|keyboardHidden"
            android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
    </application>
</manifest>
`, cfg.Package, cfg.VersionCode, cfg.VersionName, cfg.MinSDK, cfg.TargetSDK, cfg.AppName)

	return os.WriteFile(filepath.Join(cfg.WorkDir, "AndroidManifest.xml"), []byte(manifest), 0644)
}

// cmdBuildAPKFullscreen — build APK fullscreen (tanpa toolbar/splash).
func cmdBuildAPKFullscreen(targetURL, appName string) (string, error) {
	var log strings.Builder
	home, _ := os.UserHomeDir()

	versionName := "1.0.0"
	versionCode := 10000
	if idx := strings.LastIndex(appName, "@"); idx > 0 {
		v := appName[idx+1:]
		appName = appName[:idx]
		name, code, perr := parseVersion(v)
		if perr != nil {
			return "", perr
		}
		versionName = name
		versionCode = code
	}

	pkg := "com.netra." + sanitizeName(strings.ToLower(appName))
	workDir := filepath.Join(home, "apk-build", sanitizeName(appName)+"_"+fmt.Sprintf("%d", time.Now().Unix()))
	outDir := os.Getenv("NETRA_OUT_DIR")
	if outDir == "" {
		outDir = "/sdcard/NetRatest1/apk-output"
	}
	os.MkdirAll(outDir, 0755)

	iconPath := filepath.Join(home, ".netra-ai", "icon.png")
	if _, err := os.Stat(iconPath); err != nil {
		iconPath = filepath.Join(home, ".netra-ai", "icon.jpg")
		if _, err := os.Stat(iconPath); err != nil {
			iconPath = ""
		}
	}

	cfg := apkConfig{
		TargetURL:   targetURL,
		AppName:     appName,
		Package:     pkg,
		VersionName: versionName,
		VersionCode: versionCode,
		MinSDK:      24,
		TargetSDK:   34,
		WorkDir:     workDir,
		OutputDir:   outDir,
		IconPath:    iconPath,
	}

	log.WriteString(fmt.Sprintf("[BUILD] APK fullscreen: %s\n\n", appName))
	log.WriteString(fmt.Sprintf("URL : %s\n", targetURL))
	log.WriteString(fmt.Sprintf("Versi: %s (code %d)\n", versionName, versionCode))
	if iconPath != "" {
		log.WriteString(fmt.Sprintf("Icon: %s\n\n", iconPath))
	} else {
		log.WriteString("Icon: default (generate dari inisial)\n\n")
	}

	steps := []struct {
		name string
		fn   func(apkConfig) error
	}{
		{"prepare", prepareWorkdir},
		{"icon", generateIconsGo},
		{"java", writeJavaSourcesPureFullscreen},
		{"manifest", writeManifestFullscreen},
		{"styles", writeStyles},
		{"compile", compileJava},
		{"dex", convertToDex},
		{"package", packageAPK},
		{"sign", signAPK},
	}

	for _, s := range steps {
		log.WriteString(fmt.Sprintf("  -> %-10s ", s.name))
		if err := s.fn(cfg); err != nil {
			log.WriteString("[FAIL]\n\n")
			log.WriteString(fmt.Sprintf("Error di step %s:\n```\n%v\n```\n", s.name, err))
			return log.String(), err
		}
		log.WriteString("[OK]\n")
	}

	finalPath := filepath.Join(cfg.OutputDir, sanitizeName(appName)+".apk")
	if err := copyFile(filepath.Join(cfg.WorkDir, "app-signed.apk"), finalPath); err != nil {
		log.WriteString(fmt.Sprintf("\n[FAIL] Gagal copy: %v\n", err))
		return log.String(), err
	}

	fi, _ := os.Stat(finalPath)
	log.WriteString(fmt.Sprintf("\n[OK] APK siap!\n"))
	log.WriteString(fmt.Sprintf("[PATH] %s (%d KB)\n", finalPath, fi.Size()/1024))
	log.WriteString(fmt.Sprintf("[INFO] Install dari file manager -> tap file -> Install\n"))
	return log.String(), nil
}
