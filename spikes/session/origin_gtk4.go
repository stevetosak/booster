//go:build !(linux && gtk3)

package main

// pageOrigin is where Wails serves the page outside the Linux GTK3 build.
const pageOrigin = "wails://wails"
