//go:build !(linux && gtk3)

package window

// PageOrigin is where Wails serves the page outside the Linux GTK3 build.
const PageOrigin = "wails://wails"
