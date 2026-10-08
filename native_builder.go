// native_builder.go — build APK NATIVE (bukan WebView) dari template + data JSON.
// Template: notes-app (aplikasi catatan).
// Input: JSON dengan struktur {app_name, theme_color, notes:[{title,content}]}.
// Output: APK native dengan layout XML + Java beneran.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// NativeNote — 1 catatan.
type NativeNote struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// NativeConfig — struktur data JSON dari user.
type NativeConfig struct {
	AppName    string       `json:"app_name"`
	ThemeColor string       `json:"theme_color"`
	Notes      []NativeNote `json:"notes"`
}

// loadNativeConfig — baca + validasi JSON.
func loadNativeConfig(path string) (*NativeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("baca JSON gagal: %w", err)
	}
	var cfg NativeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse JSON gagal: %w", err)
	}
	if cfg.AppName == "" {
		cfg.AppName = "NetRa Notes"
	}
	if cfg.ThemeColor == "" {
		cfg.ThemeColor = "#8b5cf6"
	}
	if len(cfg.Notes) == 0 {
		return nil, fmt.Errorf("notes kosong — minimal 1")
	}
	return &cfg, nil
}

// writeFileTmp — helper nulis file.
func writeFileTmp(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// escapeXML — escape karakter XML.
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// escapeJava — escape string buat Java.
func escapeJava(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

// generateAndroidManifest — bikin manifest XML.
func generateAndroidManifest(pkg, appName string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="%s"
    android:versionCode="1"
    android:versionName="1.0">
    <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="30" />
    <application
        android:label="@string/app_name"
        android:icon="@mipmap/ic_launcher"
        android:theme="@android:style/Theme.Material.Light.NoActionBar">
        <activity
            android:name=".MainActivity"
            android:theme="@android:style/Theme.Material.Light.NoActionBar"
            android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
    </application>
</manifest>
`, pkg)
}

// generateStringsXML — strings.xml.
func generateStringsXML(appName string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">%s</string>
</resources>
`, escapeXML(appName))
}

// generateColorsXML — colors.xml.
func generateColorsXML(themeColor string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<resources>
    <color name="primary">%s</color>
    <color name="bg">#f5f5f7</color>
    <color name="card">#ffffff</color>
    <color name="text">#1a1a1a</color>
    <color name="text_muted">#6b6b6b</color>
</resources>
`, themeColor)
}

// generateActivityMainXML — layout utama (ScrollView + LinearLayout container).
func generateActivityMainXML() string {
	return `<?xml version="1.0" encoding="utf-8"?>
<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="match_parent"
    android:layout_height="match_parent"
    android:orientation="vertical"
    android:background="@color/bg">

    <TextView
        android:id="@+id/header"
        android:layout_width="match_parent"
        android:layout_height="wrap_content"
        android:background="@color/primary"
        android:padding="20dp"
        android:textColor="#ffffff"
        android:textSize="22sp"
        android:textStyle="bold"
        android:text="@string/app_name" />

    <TextView
        android:id="@+id/subheader"
        android:layout_width="match_parent"
        android:layout_height="wrap_content"
        android:padding="14dp"
        android:textColor="@color/text_muted"
        android:textSize="13sp" />

    <ScrollView
        android:layout_width="match_parent"
        android:layout_height="0dp"
        android:layout_weight="1">

        <LinearLayout
            android:id="@+id/container"
            android:layout_width="match_parent"
            android:layout_height="wrap_content"
            android:orientation="vertical"
            android:padding="12dp" />

    </ScrollView>
</LinearLayout>
`
}

// generateItemNoteXML — layout per item.
func generateItemNoteXML() string {
	return `<?xml version="1.0" encoding="utf-8"?>
<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="match_parent"
    android:layout_height="wrap_content"
    android:orientation="vertical"
    android:background="@color/card"
    android:padding="14dp"
    android:layout_marginBottom="10dp"
    android:elevation="2dp">

    <TextView
        android:id="@+id/item_title"
        android:layout_width="match_parent"
        android:layout_height="wrap_content"
        android:textColor="@color/text"
        android:textSize="16sp"
        android:textStyle="bold" />

    <TextView
        android:id="@+id/item_content"
        android:layout_width="match_parent"
        android:layout_height="wrap_content"
        android:layout_marginTop="6dp"
        android:textColor="@color/text_muted"
        android:textSize="14sp" />
</LinearLayout>
`
}

// generateMainActivityJava — Java code dengan data notes hardcoded.
// Pakai getIdentifier() biar gak tergantung R.java ID statis.
func generateMainActivityJava(pkg string, cfg *NativeConfig) string {
	// Susun array Java dari JSON
	var notesBuilder strings.Builder
	notesBuilder.WriteString("        final String[][] NOTES = {\n")
	for i, note := range cfg.Notes {
		comma := ","
		if i == len(cfg.Notes)-1 {
			comma = ""
		}
		notesBuilder.WriteString(fmt.Sprintf(
			"            {\"%s\", \"%s\"}%s\n",
			escapeJava(note.Title), escapeJava(note.Content), comma))
	}
	notesBuilder.WriteString("        };\n")

	return fmt.Sprintf(`package %s;

import android.app.Activity;
import android.os.Bundle;
import android.view.LayoutInflater;
import android.view.View;
import android.view.ViewGroup;
import android.widget.LinearLayout;
import android.widget.TextView;

public class MainActivity extends Activity {
    private int idOf(String name, String type) {
        return getResources().getIdentifier(name, type, getPackageName());
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        int layoutMain = idOf("activity_main", "layout");
        setContentView(layoutMain);

        TextView sub = findViewById(idOf("subheader", "id"));
        sub.setText("Total: " + %d + " catatan");

        LinearLayout container = findViewById(idOf("container", "id"));
        LayoutInflater inflater = getLayoutInflater();
        int itemLayout = idOf("item_note", "layout");

%s
        for (int i = 0; i < NOTES.length; i++) {
            View item = inflater.inflate(itemLayout, container, false);
            TextView t = item.findViewById(idOf("item_title", "id"));
            TextView c = item.findViewById(idOf("item_content", "id"));
            t.setText((i + 1) + ". " + NOTES[i][0]);
            c.setText(NOTES[i][1]);
            container.addView(item);
        }
    }
}
`, pkg, len(cfg.Notes), notesBuilder.String())
}

// cmdBuildNativeAPK — entry point.
// Pakai: /build native <data.json> [nama-app-override]
func cmdBuildNativeAPK(dataJSONPath, appNameOverride string) (string, error) {
	var log strings.Builder

	// Expand ~
	if strings.HasPrefix(dataJSONPath, "~/") {
		home, _ := os.UserHomeDir()
		dataJSONPath = filepath.Join(home, dataJSONPath[2:])
	}

	// Load JSON
	cfg, err := loadNativeConfig(dataJSONPath)
	if err != nil {
		return "", err
	}
	if appNameOverride != "" {
		cfg.AppName = appNameOverride
	}

	// Workdir
	home, _ := os.UserHomeDir()
	pkg := "com.netra." + sanitizeName(strings.ToLower(cfg.AppName))
	workDir := filepath.Join(home, "apk-build", sanitizeName(cfg.AppName)+"_native_"+fmt.Sprintf("%d", time.Now().Unix()))
	outDir := "/sdcard/NetRatest1/apk-output"

	os.MkdirAll(outDir, 0755)

	log.WriteString(fmt.Sprintf("🎨 **Building NATIVE APK: %s**\n\n", cfg.AppName))
	log.WriteString(fmt.Sprintf("Template: `notes-app`\n"))
	log.WriteString(fmt.Sprintf("Package : `%s`\n", pkg))
	log.WriteString(fmt.Sprintf("Notes   : %d item\n", len(cfg.Notes)))
	log.WriteString(fmt.Sprintf("Theme   : `%s`\n\n", cfg.ThemeColor))

	// 1. Bikin struktur folder
	dirs := []string{
		filepath.Join(workDir, "src", strings.ReplaceAll(pkg, ".", "/")),
		filepath.Join(workDir, "classes"),
		filepath.Join(workDir, "res", "values"),
		filepath.Join(workDir, "res", "layout"),
		filepath.Join(workDir, "res", "mipmap-mdpi"),
		filepath.Join(workDir, "res", "mipmap-hdpi"),
		filepath.Join(workDir, "res", "mipmap-xhdpi"),
		filepath.Join(workDir, "res", "mipmap-xxhdpi"),
		filepath.Join(workDir, "res", "mipmap-xxxhdpi"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return log.String(), err
		}
	}
	log.WriteString("  `→` folders      ✓\n")

	// 2. AndroidManifest.xml
	if err := writeFileTmp(filepath.Join(workDir, "AndroidManifest.xml"), generateAndroidManifest(pkg, cfg.AppName)); err != nil {
		return log.String(), err
	}
	log.WriteString("  `→` manifest     ✓\n")

	// 3. Resources XML
	srcPath := filepath.Join(workDir, "src", strings.ReplaceAll(pkg, ".", "/"))
	_ = srcPath
	if err := writeFileTmp(filepath.Join(workDir, "res", "values", "strings.xml"), generateStringsXML(cfg.AppName)); err != nil {
		return log.String(), err
	}
	if err := writeFileTmp(filepath.Join(workDir, "res", "values", "colors.xml"), generateColorsXML(cfg.ThemeColor)); err != nil {
		return log.String(), err
	}
	if err := writeFileTmp(filepath.Join(workDir, "res", "layout", "activity_main.xml"), generateActivityMainXML()); err != nil {
		return log.String(), err
	}
	if err := writeFileTmp(filepath.Join(workDir, "res", "layout", "item_note.xml"), generateItemNoteXML()); err != nil {
		return log.String(), err
	}
	log.WriteString("  `→` resources    ✓\n")

	// 4. MainActivity.java
	javaPath := filepath.Join(workDir, "src", strings.ReplaceAll(pkg, ".", "/"), "MainActivity.java")
	if err := writeFileTmp(javaPath, generateMainActivityJava(pkg, cfg)); err != nil {
		return log.String(), err
	}
	log.WriteString("  `→` java         ✓\n")

	// 5. Icon (pake generateIcons kalau ada dari apkbuilder.go)
	iconCfg := apkConfig{WorkDir: workDir, IconPath: ""}
	if err := generateIcons(iconCfg); err != nil {
		log.WriteString("  `→` icon         ⚠ (skip)\n")
	} else {
		log.WriteString("  `→` icon         ✓\n")
	}

	// 6. Compile Java
	androidJar := filepath.Join(home, "android-sdk", "android.jar")
	compileCmd := exec.Command("ecj",
		"-source", "1.7", "-target", "1.7",
		"-cp", androidJar,
		"-d", filepath.Join(workDir, "classes"),
		javaPath,
	)
	out, err := compileCmd.CombinedOutput()
	if err != nil {
		log.WriteString(fmt.Sprintf("  `→` compile      ❌\n\n```\n%s\n```\n", string(out)))
		return log.String(), err
	}
	log.WriteString("  `→` compile      ✓\n")

	// 7. Dex
	dexCmd := exec.Command("dx", "--dex",
		"--output", filepath.Join(workDir, "classes.dex"),
		filepath.Join(workDir, "classes"),
	)
	out, err = dexCmd.CombinedOutput()
	if err != nil {
		log.WriteString(fmt.Sprintf("  `→` dex          ❌\n\n```\n%s\n```\n", string(out)))
		return log.String(), err
	}
	log.WriteString("  `→` dex          ✓\n")

	// 8. Package (aapt)
	unsigned := filepath.Join(workDir, "app-unsigned.apk")
	aaptCmd := exec.Command("aapt", "package",
		"-f",
		"-M", filepath.Join(workDir, "AndroidManifest.xml"),
		"-S", filepath.Join(workDir, "res"),
		"-I", androidJar,
		"-F", unsigned,
	)
	out, err = aaptCmd.CombinedOutput()
	if err != nil {
		log.WriteString(fmt.Sprintf("  `→` package      ❌\n\n```\n%s\n```\n", string(out)))
		return log.String(), err
	}
	log.WriteString("  `→` package      ✓\n")

	// 9. Add classes.dex
	addCmd := exec.Command("aapt", "add", unsigned, "classes.dex")
	addCmd.Dir = workDir
	out, err = addCmd.CombinedOutput()
	if err != nil {
		log.WriteString(fmt.Sprintf("  `→` add dex      ❌\n\n```\n%s\n```\n", string(out)))
		return log.String(), err
	}
	log.WriteString("  `→` add dex      ✓\n")

	// 10. Sign
	keystore := filepath.Join(home, ".netra-ai", "netra.keystore")
	signed := filepath.Join(workDir, "app-signed.apk")
	signCmd := exec.Command("apksigner", "sign",
		"--ks", keystore,
		"--ks-pass", "pass:netra123",
		"--key-pass", "pass:netra123",
		"--out", signed,
		unsigned,
	)
	out, err = signCmd.CombinedOutput()
	if err != nil {
		log.WriteString(fmt.Sprintf("  `→` sign         ❌\n\n```\n%s\n```\n", string(out)))
		return log.String(), err
	}
	log.WriteString("  `→` sign         ✓\n")

	// Copy ke output
	finalPath := filepath.Join(outDir, sanitizeName(cfg.AppName)+".apk")
	if err := copyFile(signed, finalPath); err != nil {
		return log.String(), err
	}

	fi, _ := os.Stat(finalPath)
	log.WriteString(fmt.Sprintf("\n✅ **APK native siap!**\n"))
	log.WriteString(fmt.Sprintf("📁 `%s` (%d KB)\n", finalPath, fi.Size()/1024))
	log.WriteString("🎨 Native UI — bukan WebView\n")
	return log.String(), nil
}
