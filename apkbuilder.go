// apkbuilder.go — build APK dari URL pakai tool native Termux.
// Support: custom icon + splash screen.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type apkConfig struct {
	TargetURL   string
	AppName     string
	Package     string
	VersionName string
	VersionCode int
	MinSDK      int
	TargetSDK   int
	WorkDir     string
	OutputDir   string
	IconPath    string
}

func cmdBuildAPKFromURL(targetURL, appName string) (string, error) {
	var log strings.Builder
	home, _ := os.UserHomeDir()
	// Parse @version dari nama app: "MyApp@1.2.3"
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
	outDir := "/sdcard/NetRatest1/apk-output"
	os.MkdirAll(outDir, 0755)

	// Cari icon custom
	iconPath := filepath.Join(home, ".netra-ai", "icon.png")
	if _, err := os.Stat(iconPath); err != nil {
		iconPath = filepath.Join(home, ".netra-ai", "icon.jpg")
		if _, err := os.Stat(iconPath); err != nil {
			iconPath = "" // gak ada, pake default
		}
	}

	cfg := apkConfig{
		TargetURL:   targetURL,
		AppName:     appName,
		Package:     pkg,
		VersionName: versionName,
		VersionCode: versionCode,
		MinSDK:      21,
		TargetSDK:   30,
		WorkDir:     workDir,
		OutputDir:   outDir,
		IconPath:    iconPath,
	}

	log.WriteString(fmt.Sprintf("📦 **Building APK: %s**\n\n", appName))
	log.WriteString(fmt.Sprintf("URL : `%s`\n", targetURL))
	log.WriteString(fmt.Sprintf("Versi: `%s` (code %d)\n", versionName, versionCode))
	if iconPath != "" {
		log.WriteString(fmt.Sprintf("Icon: `%s`\n\n", iconPath))
	} else {
		log.WriteString("Icon: default (generate dari inisial)\n\n")
	}

	steps := []struct {
		name string
		fn   func(apkConfig) error
	}{
		{"prepare", prepareWorkdir},
		{"icon", generateIcons},
		{"java", writeJavaSources},
		{"manifest", writeManifest},
		{"styles", writeStyles},
		{"compile", compileJava},
		{"dex", convertToDex},
		{"package", packageAPK},
		{"sign", signAPK},
	}

	for _, s := range steps {
		log.WriteString(fmt.Sprintf("  `→` %-10s ", s.name))
		if err := s.fn(cfg); err != nil {
			log.WriteString("❌\n\n")
			log.WriteString(fmt.Sprintf("**Error di step `%s`:**\n```\n%v\n```\n", s.name, err))
			return log.String(), err
		}
		log.WriteString("✓\n")
	}

	finalPath := filepath.Join(cfg.OutputDir, sanitizeName(appName)+".apk")
	if err := copyFile(filepath.Join(cfg.WorkDir, "app-signed.apk"), finalPath); err != nil {
		log.WriteString(fmt.Sprintf("\n❌ Gagal copy: %v\n", err))
		return log.String(), err
	}

	fi, _ := os.Stat(finalPath)
	log.WriteString(fmt.Sprintf("\n✅ **APK siap!**\n"))
	log.WriteString(fmt.Sprintf("📁 `%s` (%d KB)\n", finalPath, fi.Size()/1024))
	log.WriteString(fmt.Sprintf("📲 Install dari file manager → tap file → Install\n"))
	return log.String(), nil
}

func prepareWorkdir(cfg apkConfig) error {
	pkgPath := strings.ReplaceAll(cfg.Package, ".", "/")
	dirs := []string{
		filepath.Join(cfg.WorkDir, "src", pkgPath),
		filepath.Join(cfg.WorkDir, "classes"),
		filepath.Join(cfg.WorkDir, "res", "values"),
		filepath.Join(cfg.WorkDir, "res", "xml"),
		filepath.Join(cfg.WorkDir, "res", "mipmap-mdpi"),
		filepath.Join(cfg.WorkDir, "res", "mipmap-hdpi"),
		filepath.Join(cfg.WorkDir, "res", "mipmap-xhdpi"),
		filepath.Join(cfg.WorkDir, "res", "mipmap-xxhdpi"),
		filepath.Join(cfg.WorkDir, "res", "mipmap-xxxhdpi"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	return nil
}

// generateIcons — resize icon ke 5 ukuran mipmap Android.
func generateIcons(cfg apkConfig) error {
	sizes := map[string]int{
		"mipmap-mdpi":    48,
		"mipmap-hdpi":    72,
		"mipmap-xhdpi":   96,
		"mipmap-xxhdpi":  144,
		"mipmap-xxxhdpi": 192,
	}
	adaptiveSizes := map[string]int{
		"mipmap-mdpi":    108,
		"mipmap-hdpi":    162,
		"mipmap-xhdpi":   216,
		"mipmap-xxhdpi":  324,
		"mipmap-xxxhdpi": 432,
	}

	// Kalau gak ada icon custom, generate sederhana pakai ImageMagick
	if cfg.IconPath == "" {
		defaultIcon := filepath.Join(cfg.WorkDir, "default-icon.png")
		initial := "N"
		for _, r := range cfg.AppName {
			if r != ' ' {
				initial = strings.ToUpper(string(r))
				break
			}
		}
		cmd := exec.Command("convert",
			"-size", "512x512",
			"xc:#8b5cf6",
			"-fill", "white",
			"-gravity", "center",
			"-pointsize", "300",
			"-font", "DejaVu-Sans-Bold",
			"-annotate", "+0+20", initial,
			defaultIcon,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			_ = out
			return fmt.Errorf("gagal generate default icon: %w", err)
		}
		cfg.IconPath = defaultIcon
	}

	// 1. Legacy icons (Android < 8)
	for dir, size := range sizes {
		outPath := filepath.Join(cfg.WorkDir, "res", dir, "ic_launcher.png")
		cmd := exec.Command("convert", cfg.IconPath,
			"-resize", fmt.Sprintf("%dx%d", size, size),
			"-background", "none",
			"-gravity", "center",
			"-extent", fmt.Sprintf("%dx%d", size, size),
			outPath,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			_ = out
			return fmt.Errorf("gagal resize icon %s: %w", dir, err)
		}
	}

	// 2. Adaptive icon foreground (108dp canvas, content ~66%)
	for dir, size := range adaptiveSizes {
		content := size * 2 / 3
		outPath := filepath.Join(cfg.WorkDir, "res", dir, "ic_launcher_foreground.png")
		cmd := exec.Command("convert", cfg.IconPath,
			"-resize", fmt.Sprintf("%dx%d", content, content),
			"-background", "none",
			"-gravity", "center",
			"-extent", fmt.Sprintf("%dx%d", size, size),
			outPath,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			_ = out
			return fmt.Errorf("gagal resize adaptive icon %s: %w", dir, err)
		}
	}

	// 3. Adaptive icon descriptor (XML)
	anydpiDir := filepath.Join(cfg.WorkDir, "res", "mipmap-anydpi-v26")
	if err := os.MkdirAll(anydpiDir, 0755); err != nil {
		return err
	}
	adaptiveXML := `<?xml version="1.0" encoding="utf-8"?>
<adaptive-icon xmlns:android="http://schemas.android.com/apk/res/android">
    <background android:drawable="@color/ic_launcher_background"/>
    <foreground android:drawable="@mipmap/ic_launcher_foreground"/>
</adaptive-icon>
`
	if err := os.WriteFile(filepath.Join(anydpiDir, "ic_launcher.xml"), []byte(adaptiveXML), 0644); err != nil {
		return err
	}

	// 4. Background color resource
	valuesDir := filepath.Join(cfg.WorkDir, "res", "values")
	if err := os.MkdirAll(valuesDir, 0755); err != nil {
		return err
	}
	colorsXML := `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <color name="ic_launcher_background">#8b5cf6</color>
</resources>
`
	if err := os.WriteFile(filepath.Join(valuesDir, "colors.xml"), []byte(colorsXML), 0644); err != nil {
		return err
	}

	return nil
}

// writeJavaSources — tulis 2 file: SplashActivity + MainActivity.
func writeJavaSources(cfg apkConfig) error {
	pkgPath := strings.ReplaceAll(cfg.Package, ".", "/")
	srcDir := filepath.Join(cfg.WorkDir, "src", pkgPath)

	// SplashActivity — layar loading 1.5 detik dengan icon
	splashSrc := fmt.Sprintf(`package %s;

import android.app.Activity;
import android.os.Bundle;
import android.os.Handler;
import android.content.Intent;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.view.Gravity;
import android.graphics.Color;

public class SplashActivity extends Activity {
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        LinearLayout ll = new LinearLayout(this);
        ll.setBackgroundColor(Color.parseColor("#120d1f"));
        ll.setGravity(Gravity.CENTER);
        ll.setOrientation(LinearLayout.VERTICAL);

        ImageView iv = new ImageView(this);
        iv.setImageResource(getResources().getIdentifier("ic_launcher", "mipmap", getPackageName()));
        ll.addView(iv);

        setContentView(ll);

        new Handler().postDelayed(new Runnable() {
            public void run() {
                startActivity(new Intent(SplashActivity.this, MainActivity.class));
                finish();
            }
        }, 1500);
    }
}
`, cfg.Package)

	if err := os.WriteFile(filepath.Join(srcDir, "SplashActivity.java"), []byte(splashSrc), 0644); err != nil {
		return err
	}

	// MainActivity — WebView + toolbar + offline cache
	mainSrc := fmt.Sprintf(`package %s;

import android.app.Activity;
import android.os.Bundle;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.webkit.WebChromeClient;
import android.webkit.WebSettings;
import android.net.Uri;
import android.content.Intent;
import android.content.ActivityNotFoundException;
import android.view.KeyEvent;
import android.view.View;
import android.view.Gravity;
import android.widget.Button;
import android.widget.ProgressBar;
import android.widget.LinearLayout;
import android.widget.TextView;
import android.graphics.Color;

public class MainActivity extends Activity {
    private WebView webView;
    private Button btnBack, btnFwd, btnRefresh, btnHome;
    private ProgressBar progressBar;
    private String homeUrl;
    private String homeHost;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        homeUrl = "__TARGETURL__";
        try {
            homeHost = Uri.parse(homeUrl).getHost();
        } catch (Exception e) {
            homeHost = null;
        }

        LinearLayout root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);

        LinearLayout toolbar = new LinearLayout(this);
        toolbar.setOrientation(LinearLayout.HORIZONTAL);
        toolbar.setBackgroundColor(Color.parseColor("#120d1f"));
        toolbar.setGravity(Gravity.CENTER_VERTICAL);

        btnBack = makeBtn("\u25C0");
        btnFwd = makeBtn("\u25B6");
        btnHome = makeBtn("\u2302");
        btnRefresh = makeBtn("\u27F3");

        TextView titleView = new TextView(this);
        titleView.setText("__APPNAME__");
        titleView.setTextColor(Color.parseColor("#ece9f7"));
        titleView.setTextSize(16);
        titleView.setSingleLine(true);
        titleView.setEllipsize(android.text.TextUtils.TruncateAt.END);
        LinearLayout.LayoutParams titleParams = new LinearLayout.LayoutParams(
            0, LinearLayout.LayoutParams.WRAP_CONTENT, 3.0f);
        titleParams.setMargins(16, 0, 16, 0);
        titleView.setLayoutParams(titleParams);
        titleView.setGravity(Gravity.CENTER);

        toolbar.addView(btnBack);
        toolbar.addView(btnFwd);
        toolbar.addView(btnHome);
        toolbar.addView(titleView);
        toolbar.addView(btnRefresh);

        progressBar = new ProgressBar(this, null, android.R.attr.progressBarStyleHorizontal);
        progressBar.setMax(100);
        progressBar.setVisibility(View.GONE);
        progressBar.setIndeterminate(false);

        webView = new WebView(this);
        WebSettings s = webView.getSettings();
        s.setJavaScriptEnabled(true);
        s.setDomStorageEnabled(true);
        s.setDatabaseEnabled(true);
        s.setLoadWithOverviewMode(true);
        s.setUseWideViewPort(true);
        s.setBuiltInZoomControls(true);
        s.setDisplayZoomControls(false);
        s.setCacheMode(WebSettings.LOAD_CACHE_ELSE_NETWORK);
        s.setAppCacheEnabled(true);
        s.setUserAgentString("Mozilla/5.0 (Linux; Android 13; SM-S908B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36");
        s.setMixedContentMode(WebSettings.MIXED_CONTENT_ALWAYS_ALLOW);
        s.setAllowFileAccess(true);
        s.setAllowContentAccess(true);
        s.setJavaScriptCanOpenWindowsAutomatically(true);
        s.setSupportMultipleWindows(false);
        s.setMediaPlaybackRequiresUserGesture(false);

        webView.setWebViewClient(new WebViewClient() {
            @Override
            public void onPageFinished(WebView view, String url) {
                btnBack.setEnabled(view.canGoBack());
                btnFwd.setEnabled(view.canGoForward());
                injectAdblock(view);
            }

            @Override
            public boolean shouldOverrideUrlLoading(WebView view, String url) {
                Uri u = Uri.parse(url);
                String scheme = u.getScheme();
                String host = u.getHost();

                // Non-HTTP (mailto, tel, whatsapp, dll) -> lempar ke app
                if (scheme != null && !scheme.equals("http") && !scheme.equals("https")) {
                    try {
                        startActivity(new Intent(Intent.ACTION_VIEW, u));
                    } catch (Exception e) {}
                    return true;
                }

                // Domain sama atau subdomain -> tetap di WebView
                if (host != null && homeHost != null && host.endsWith(homeHost)) {
                    return false;
                }

                // Domain luar -> buka di browser HP
                try {
                    startActivity(new Intent(Intent.ACTION_VIEW, u));
                    return true;
                } catch (ActivityNotFoundException e) {
                    return false;
                }
            }
        });

        webView.setWebChromeClient(new WebChromeClient() {
            @Override
            public void onProgressChanged(WebView view, int newProgress) {
                if (newProgress < 100) {
                    progressBar.setVisibility(View.VISIBLE);
                    progressBar.setProgress(newProgress);
                } else {
                    progressBar.setVisibility(View.GONE);
                }
            }
        });

        btnBack.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) { if (webView.canGoBack()) webView.goBack(); }
        });
        btnFwd.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) { if (webView.canGoForward()) webView.goForward(); }
        });
        btnHome.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) { webView.loadUrl(homeUrl); }
        });
        btnRefresh.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) { webView.reload(); }
        });

        webView.loadUrl(homeUrl);

        LinearLayout.LayoutParams wvParams = new LinearLayout.LayoutParams(
            LinearLayout.LayoutParams.MATCH_PARENT, 0, 1.0f);
        root.addView(toolbar);
        root.addView(progressBar, new LinearLayout.LayoutParams(
            LinearLayout.LayoutParams.MATCH_PARENT, 8));
        root.addView(webView, wvParams);

        setContentView(root);
    }

    private Button makeBtn(String text) {
        Button b = new Button(this);
        b.setText(text);
        b.setTextSize(16);
        b.setBackgroundColor(Color.parseColor("#1a1330"));
        b.setTextColor(Color.parseColor("#ece9f7"));
        LinearLayout.LayoutParams p = new LinearLayout.LayoutParams(
            0, LinearLayout.LayoutParams.WRAP_CONTENT, 1.0f);
        b.setLayoutParams(p);
        return b;
    }

    private void injectAdblock(WebView view) {
        String js = "(function(){" +
            "var css='div[class*=\"ad-\"],div[class*=\"-ad\"],div[class*=\"ads\"],div[id*=\"ad-\"],div[id*=\"ads\"],div[class*=\"banner\"],div[class*=\"popup\"],div[class*=\"overlay\"],div[class*=\"sponsor\"],div[class*=\"promo\"],iframe[src*=\"googlesyndication\"],iframe[src*=\"doubleclick\"],iframe[src*=\"googleads\"],iframe[src*=\"adservice\"]{display:none !important;visibility:hidden !important}';" +
            "var s=document.createElement('style');s.type='text/css';s.appendChild(document.createTextNode(css));document.head.appendChild(s);" +
            "var kill=function(){var els=document.querySelectorAll('[class*=\"ad-\"],[class*=\"-ad\"],[class*=\"ads\"],[class*=\"banner\"],[class*=\"popup\"],[class*=\"overlay\"],[class*=\"sponsor\"],[class*=\"promo\"]');for(var i=0;i<els.length;i++){els[i].style.display='none'}};kill();setTimeout(kill,1500);setTimeout(kill,4000);" +
            "})();";
        view.evaluateJavascript(js, null);
    }

    @Override
    public boolean onKeyDown(int keyCode, KeyEvent event) {
        if (keyCode == KeyEvent.KEYCODE_BACK && webView.canGoBack()) {
            webView.goBack();
            return true;
        }
        return super.onKeyDown(keyCode, event);
    }
}
`, cfg.Package)

	// Replace placeholder — biar gak mungkin ketuker
	mainSrc = strings.ReplaceAll(mainSrc, "__APPNAME__", cfg.AppName)
	mainSrc = strings.ReplaceAll(mainSrc, "__TARGETURL__", cfg.TargetURL)
	return os.WriteFile(filepath.Join(srcDir, "MainActivity.java"), []byte(mainSrc), 0644)
}

func writeManifest(cfg apkConfig) error {
	manifest := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="%s"
    android:versionCode="%d"
    android:versionName="%s">

    <uses-sdk android:minSdkVersion="%d" android:targetSdkVersion="%d" />
    <uses-permission android:name="android.permission.INTERNET" />
    <uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />

    <application
        android:label="%s"
        android:icon="@mipmap/ic_launcher"
        android:usesCleartextTraffic="true"
        android:networkSecurityConfig="@xml/network_security_config"
        android:debuggable="false"
        android:allowBackup="false"
        android:supportsRtl="true">
        <activity
            android:name=".SplashActivity"
            android:theme="@android:style/Theme.NoTitleBar"
            android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
        <activity
            android:name=".MainActivity"
            android:theme="@android:style/Theme.NoTitleBar"
            android:exported="false" />
    </application>
</manifest>
`, cfg.Package, cfg.VersionCode, cfg.VersionName, cfg.MinSDK, cfg.TargetSDK, cfg.AppName)

	return os.WriteFile(filepath.Join(cfg.WorkDir, "AndroidManifest.xml"), []byte(manifest), 0644)
}

func writeStyles(cfg apkConfig) error {
	// Bikin network_security_config.xml biar WebView boleh akses 127.0.0.1
	nsc := `<?xml version="1.0" encoding="utf-8"?>
<network-security-config>
    <domain-config cleartextTrafficPermitted="true">
        <domain includeSubdomains="true">127.0.0.1</domain>
        <domain includeSubdomains="true">localhost</domain>
    </domain-config>
    <base-config cleartextTrafficPermitted="true" />
</network-security-config>
`
	return os.WriteFile(filepath.Join(cfg.WorkDir, "res", "xml", "network_security_config.xml"), []byte(nsc), 0644)
}

func compileJava(cfg apkConfig) error {
	home, _ := os.UserHomeDir()
	androidJar := filepath.Join(home, "android-sdk", "android.jar")
	pkgPath := strings.ReplaceAll(cfg.Package, ".", "/")
	srcDir := filepath.Join(cfg.WorkDir, "src", pkgPath)
	classesDir := filepath.Join(cfg.WorkDir, "classes")

	// Cari semua .java
	var javaFiles []string
	entries, _ := os.ReadDir(srcDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".java") {
			javaFiles = append(javaFiles, filepath.Join(srcDir, e.Name()))
		}
	}

	args := []string{
		"-source", "1.7", "-target", "1.7",
		"-cp", androidJar,
		"-d", classesDir,
	}
	args = append(args, javaFiles...)

	cmd := exec.Command("ecj", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = out
	}
	return err
}

func convertToDex(cfg apkConfig) error {
	classesDir := filepath.Join(cfg.WorkDir, "classes")
	outDex := filepath.Join(cfg.WorkDir, "classes.dex")

	cmd := exec.Command("dx", "--dex", "--output", outDex, classesDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = out
	}
	return err
}

func packageAPK(cfg apkConfig) error {
	home, _ := os.UserHomeDir()
	androidJar := filepath.Join(home, "android-sdk", "android.jar")
	unsignedAPK := filepath.Join(cfg.WorkDir, "app-unsigned.apk")
	resDir := filepath.Join(cfg.WorkDir, "res")

	// Step 1: aapt package (dengan resources)
	cmd1 := exec.Command("aapt", "package",
		"-f",
		"-M", filepath.Join(cfg.WorkDir, "AndroidManifest.xml"),
		"-S", resDir,
		"-I", androidJar,
		"-F", unsignedAPK,
	)
	out, err := cmd1.CombinedOutput()
	if err != nil {
		return fmt.Errorf("aapt package: %v\n%s", err, string(out))
	}

	// Step 2: aapt add (tambahin classes.dex)
	cmd2 := exec.Command("aapt", "add", unsignedAPK, "classes.dex")
	cmd2.Dir = cfg.WorkDir
	out2, err := cmd2.CombinedOutput()
	if err != nil {
		return fmt.Errorf("aapt add: %v\n%s", err, string(out2))
	}
	return nil
}

func signAPK(cfg apkConfig) error {
	home, _ := os.UserHomeDir()
	keystore := filepath.Join(home, ".netra-ai", "netra.keystore")
	os.MkdirAll(filepath.Dir(keystore), 0755)

	if _, err := os.Stat(keystore); os.IsNotExist(err) {
		_ = "keystore-generate"
		if err := generateKeystore(keystore); err != nil {
			return err
		}
	}

	unsignedAPK := filepath.Join(cfg.WorkDir, "app-unsigned.apk")
	signedAPK := filepath.Join(cfg.WorkDir, "app-signed.apk")

	cmd := exec.Command("apksigner", "sign",
		"--ks", keystore,
		"--ks-pass", "pass:netra123",
		"--key-pass", "pass:netra123",
		"--v1-signing-enabled", "true",
		"--v2-signing-enabled", "true",
		"--v3-signing-enabled", "true",
		"--out", signedAPK,
		unsignedAPK,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = out
	}
	return err
}

func generateKeystore(keystore string) error {
	cmd := exec.Command("keytool", "-genkeypair",
		"-keystore", keystore,
		"-alias", "netra",
		"-keyalg", "RSA",
		"-keysize", "2048",
		"-validity", "10000",
		"-storepass", "netra123",
		"-keypass", "netra123",
		"-dname", "CN=NetRa, OU=Dev, O=NetRa, L=Jakarta, C=ID",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = out
	}
	return err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// parseVersion — parse "1.2.3" jadi (name, code).
// Version code = major*10000 + minor*100 + patch
func parseVersion(s string) (string, int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "1.0.0", 10000, nil
	}
	parts := strings.Split(s, ".")
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	if len(parts) > 3 {
		parts = parts[:3]
	}
	var nums [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return "", 0, fmt.Errorf("versi tidak valid: %s (harus format X.Y.Z)", s)
		}
		nums[i] = n
	}
	name := fmt.Sprintf("%d.%d.%d", nums[0], nums[1], nums[2])
	code := nums[0]*10000 + nums[1]*100 + nums[2]
	return name, code, nil
}
