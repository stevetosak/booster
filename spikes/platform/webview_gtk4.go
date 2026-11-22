//go:build linux && !gtk3

package main

// webview names the WebKitGTK that this build links: the GTK4 API, Wails' default.
var webview = webviewBuild{API: "webkitgtk-6.0 (GTK4)", Package: "libwebkitgtk-6.0-4", Origin: "wails://wails"}
