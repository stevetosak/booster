// Package webviewgpu makes WebKit draw on the integrated GPU of a hybrid
// (PRIME) laptop. Import it for its side effect only.
//
// glvnd tries NVIDIA's EGL before Mesa's, so WebKit's web process draws on the
// discrete GPU, and Wails then turns WebKit's DMA-BUF renderer off on X11
// whenever the nvidia module is loaded. Inside a Wails window that renderer off
// shows each update one update late. Limiting EGL and GLX to Mesa lets the
// integrated GPU draw, and Wails' own rule then keeps DMA-BUF on.
//
// Wails reads the environment in its package init. Go initializes packages in
// the order of their import paths, each after its own imports, so this
// package (booster/..., importing only the standard library) runs before
// github.com/wailsapp/... does.
package webviewgpu

import (
	"os"
	"path/filepath"
)

// Env names the variable that turns the choice off: "system" leaves GL to the
// system and Wails.
const Env = "BOOSTER_WEBVIEW_GPU"

// original is the environment as the process got it, before this package
// and Wails changed it.
var original []string

// OriginalEnviron returns the environment as the process got it, before this
// package and Wails' init changed GL and WebKit variables. A child that is not
// the webview (the core, an agent) gets this one.
func OriginalEnviron() []string { return append([]string(nil), original...) }

const mesaEGLVendor = "/usr/share/glvnd/egl_vendor.d/50_mesa.json"

// integratedDrivers are the kernel drivers of integrated GPUs that Mesa drives.
var integratedDrivers = map[string]bool{"amdgpu": true, "i915": true, "xe": true}

func init() {
	original = os.Environ()
	if os.Getenv(Env) == "system" || chosenByUser() {
		return
	}
	if !hasIntegratedGPU("/sys/class/drm") || !exists(mesaEGLVendor) {
		return // with no integrated GPU, Mesa would draw on the CPU
	}
	_ = os.Setenv("__EGL_VENDOR_LIBRARY_FILENAMES", mesaEGLVendor)
	_ = os.Setenv("__GLX_VENDOR_LIBRARY_NAME", "mesa")
}

// chosenByUser reports whether the user already chose a GL vendor or the
// discrete GPU.
func chosenByUser() bool {
	for _, k := range []string{"__EGL_VENDOR_LIBRARY_FILENAMES", "__GLX_VENDOR_LIBRARY_NAME", "__NV_PRIME_RENDER_OFFLOAD"} {
		if os.Getenv(k) != "" {
			return true
		}
	}
	return false
}

func hasIntegratedGPU(drm string) bool {
	cards, _ := filepath.Glob(filepath.Join(drm, "card[0-9]*"))
	for _, card := range cards {
		driver, err := filepath.EvalSymlinks(filepath.Join(card, "device", "driver"))
		if err == nil && integratedDrivers[filepath.Base(driver)] {
			return true
		}
	}
	return false
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
