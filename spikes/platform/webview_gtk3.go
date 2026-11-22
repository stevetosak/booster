//go:build linux && gtk3

package main

// webview names the WebKitGTK that this build links: the GTK3 API.
var webview = webviewBuild{API: "webkit2gtk-4.1 (GTK3)", Package: "libwebkit2gtk-4.1-0"}
