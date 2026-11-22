//go:build linux && gtk3

package window

// PageOrigin is where Wails serves the page in the GTK3 build; the core lets
// in only this origin over TCP.
const PageOrigin = "wails://localhost"
