package com.netra.template;

import android.app.Activity;
import android.content.ActivityNotFoundException;
import android.content.Intent;
import android.graphics.Color;
import android.net.Uri;
import android.content.pm.PackageManager;
import android.os.Build;
import android.os.Bundle;
import android.view.WindowManager;
import android.webkit.JavascriptInterface;
import android.webkit.ValueCallback;
import android.webkit.WebChromeClient;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;

import java.io.ByteArrayOutputStream;
import java.io.InputStream;

public class MainActivity extends Activity {
    private static final int REQ_FILE = 4711;

    private WebView webView;
    private ValueCallback<Uri[]> fileCallback;

    private static final String JS_COLOR =
        "(function(){try{var c=null;"
        + "var m=document.querySelector('meta[name=\"theme-color\"]');"
        + "if(m&&m.content)c=m.content;"
        + "if(!c){var b=window.getComputedStyle(document.body).backgroundColor;"
        + "if(b&&b!=='rgba(0, 0, 0, 0)'&&b!=='transparent')c=b;}"
        + "if(!c){var h=window.getComputedStyle(document.documentElement).backgroundColor;"
        + "if(h&&h!=='rgba(0, 0, 0, 0)'&&h!=='transparent')c=h;}"
        + "if(c)NetraBridge.setStatusBarColor(c);}catch(e){}})()";

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        webView = new WebView(this);
        webView.setBackgroundColor(Color.BLACK);
        webView.setOverScrollMode(WebView.OVER_SCROLL_NEVER);
        setContentView(webView);

        WebSettings s = webView.getSettings();
        s.setJavaScriptEnabled(true);
        s.setDomStorageEnabled(true);
        s.setLoadWithOverviewMode(true);
        s.setUseWideViewPort(true);
        s.setSupportZoom(false);
        s.setBuiltInZoomControls(false);
        s.setDisplayZoomControls(false);
        s.setCacheMode(WebSettings.LOAD_DEFAULT);
        s.setMixedContentMode(WebSettings.MIXED_CONTENT_ALWAYS_ALLOW);
        s.setAllowFileAccess(true);
        s.setAllowContentAccess(true);
        s.setAllowFileAccessFromFileURLs(true);
        s.setAllowUniversalAccessFromFileURLs(true);
        s.setMediaPlaybackRequiresUserGesture(false);

        webView.addJavascriptInterface(new Bridge(), "NetraBridge");

        webView.setWebViewClient(new WebViewClient() {
            @Override
            public void onPageFinished(WebView view, String url) {
                view.evaluateJavascript(JS_COLOR, null);
            }

            @Override
            public boolean shouldOverrideUrlLoading(WebView view, String url) {
                Uri u = Uri.parse(url);
                String sch = u.getScheme();
                if (sch == null || sch.equals("http") || sch.equals("https")
                        || sch.equals("file") || sch.equals("about")
                        || sch.equals("blob") || sch.equals("data")) {
                    return false;
                }
                try {
                    startActivity(new Intent(Intent.ACTION_VIEW, u));
                } catch (Exception e) {
                }
                return true;
            }
        });

        // Tombol "Pilih file" di halaman web (<input type="file">).
        webView.setWebChromeClient(new WebChromeClient() {
            @Override
            public boolean onShowFileChooser(WebView view, ValueCallback<Uri[]> callback,
                                             FileChooserParams params) {
                if (fileCallback != null) {
                    fileCallback.onReceiveValue(null);
                }
                fileCallback = callback;

                Intent i = new Intent(Intent.ACTION_GET_CONTENT);
                i.addCategory(Intent.CATEGORY_OPENABLE);
                String type = "*/*";
                String[] acc = params.getAcceptTypes();
                if (acc != null && acc.length > 0 && acc[0] != null && acc[0].length() > 0) {
                    type = acc[0];
                    if (type.startsWith(".")) {
                        type = "*/*";
                    }
                }
                i.setType(type);
                if (params.getMode() == FileChooserParams.MODE_OPEN_MULTIPLE) {
                    i.putExtra(Intent.EXTRA_ALLOW_MULTIPLE, true);
                }
                try {
                    startActivityForResult(Intent.createChooser(i, "Pilih file"), REQ_FILE);
                } catch (ActivityNotFoundException e) {
                    fileCallback = null;
                    callback.onReceiveValue(null);
                    return false;
                }
                return true;
            }
        });

        askMediaPermission();
        webView.loadUrl(readConfigUrl());
    }

    // Popup izin akses musik/audio waktu app pertama dibuka.
    private void askMediaPermission() {
        if (Build.VERSION.SDK_INT < 23) {
            return;
        }
        String perm = Build.VERSION.SDK_INT >= 33
            ? "android.permission.READ_MEDIA_AUDIO"
            : "android.permission.READ_EXTERNAL_STORAGE";
        if (checkSelfPermission(perm) != PackageManager.PERMISSION_GRANTED) {
            requestPermissions(new String[] { perm }, 99);
        }
    }

    private String readConfigUrl() {
        String url = "https://example.com";
        try {
            InputStream in = getAssets().open("config.json");
            ByteArrayOutputStream out = new ByteArrayOutputStream();
            byte[] buf = new byte[1024];
            int n;
            while ((n = in.read(buf)) > 0) {
                out.write(buf, 0, n);
            }
            in.close();
            String json = out.toString("UTF-8");
            int k = json.indexOf("\"url\"");
            if (k >= 0) {
                int c = json.indexOf(':', k);
                int a = json.indexOf('"', c + 1);
                int b = a;
                // cari tanda kutip penutup, lewati yang di-escape
                do {
                    b = json.indexOf('"', b + 1);
                } while (b > 0 && json.charAt(b - 1) == '\\');
                if (a >= 0 && b > a) {
                    url = json.substring(a + 1, b).replace("\\\"", "\"").replace("\\\\", "\\");
                }
            }
        } catch (Exception e) {
        }
        return url;
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        if (requestCode == REQ_FILE) {
            if (fileCallback != null) {
                Uri[] result = null;
                if (resultCode == RESULT_OK && data != null) {
                    if (data.getClipData() != null) {
                        int n = data.getClipData().getItemCount();
                        result = new Uri[n];
                        for (int i = 0; i < n; i++) {
                            result[i] = data.getClipData().getItemAt(i).getUri();
                        }
                    } else if (data.getData() != null) {
                        result = new Uri[] { data.getData() };
                    }
                }
                fileCallback.onReceiveValue(result);
                fileCallback = null;
            }
            return;
        }
        super.onActivityResult(requestCode, resultCode, data);
    }

    @Override
    public void onBackPressed() {
        if (webView != null && webView.canGoBack()) {
            webView.goBack();
        } else {
            super.onBackPressed();
        }
    }

    @Override
    protected void onPause() {
        super.onPause();
        if (webView != null) {
            webView.onPause();
        }
    }

    @Override
    protected void onResume() {
        super.onResume();
        if (webView != null) {
            webView.onResume();
        }
    }

    private static int parseCssColor(String c) {
        c = c.trim();
        if (c.startsWith("rgb")) {
            int a = c.indexOf('(');
            int b = c.indexOf(')');
            String[] p = c.substring(a + 1, b).split(",");
            return Color.rgb(
                (int) Float.parseFloat(p[0].trim()),
                (int) Float.parseFloat(p[1].trim()),
                (int) Float.parseFloat(p[2].trim()));
        }
        return Color.parseColor(c);
    }

    public class Bridge {
        @JavascriptInterface
        public void setStatusBarColor(final String colorStr) {
            runOnUiThread(new Runnable() {
                public void run() {
                    try {
                        int col = parseCssColor(colorStr);
                        getWindow().clearFlags(WindowManager.LayoutParams.FLAG_TRANSLUCENT_STATUS);
                        getWindow().addFlags(WindowManager.LayoutParams.FLAG_DRAWS_SYSTEM_BAR_BACKGROUNDS);
                        getWindow().setStatusBarColor(col);
                        int lum = (Color.red(col) * 299 + Color.green(col) * 587 + Color.blue(col) * 114) / 1000;
                        if (android.os.Build.VERSION.SDK_INT >= 23) {
                            int flags = getWindow().getDecorView().getSystemUiVisibility();
                            if (lum > 160) {
                                flags |= android.view.View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR;
                            } else {
                                flags &= ~android.view.View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR;
                            }
                            getWindow().getDecorView().setSystemUiVisibility(flags);
                        }
                    } catch (Exception e) {
                    }
                }
            });
        }
    }
}
