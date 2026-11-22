package webviewgpu

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeDRM makes a sysfs-like drm folder whose cards link to the given drivers.
func fakeDRM(t *testing.T, drivers ...string) string {
	t.Helper()
	root := t.TempDir()
	drm := filepath.Join(root, "class", "drm")
	for i, driver := range drivers {
		target := filepath.Join(root, "bus", "pci", "drivers", driver)
		device := filepath.Join(drm, "card"+string(rune('0'+i)), "device")
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(device, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(device, "driver")); err != nil {
			t.Fatal(err)
		}
	}
	return drm
}

func TestHasIntegratedGPU(t *testing.T) {
	cases := []struct {
		name    string
		drivers []string
		want    bool
	}{
		{"AMD and NVIDIA (PRIME)", []string{"amdgpu", "nvidia"}, true},
		{"Intel and NVIDIA", []string{"nvidia", "i915"}, true},
		{"NVIDIA only", []string{"nvidia"}, false},
		{"no GPU", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hasIntegratedGPU(fakeDRM(t, c.drivers...)); got != c.want {
				t.Errorf("hasIntegratedGPU = %v, want %v", got, c.want)
			}
		})
	}
}
