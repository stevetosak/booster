package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// gpuLibrary matches the GL and EGL driver libraries a process maps. WebKit
// hides the GPU from the page (WebGL reports "Apple GPU"), so these libraries
// are what shows which driver renders (radeonsi, NVIDIA, or swrast, the CPU)
// and whether the DMA-BUF path (gbm) is in use.
var gpuLibrary = regexp.MustCompile(`^(\w+_dri\.so|libnvidia-(glcore|eglcore|glsi|egl-gbm)\.so.*|lib(EGL|GLX)_(nvidia|mesa)\.so.*|libgbm\.so.*|libgallium.*\.so)$`)

// processGPU is the GPU driver libraries one process has loaded.
type processGPU struct {
	PID       int      `json:"pid"`
	Name      string   `json:"name"`
	Libraries []string `json:"libraries"`
}

// readGPU lists the driver libraries of this process and WebKit's helpers.
func readGPU(procs []processMemory) []processGPU {
	self := os.Getpid()
	var out []processGPU
	for _, p := range procs {
		if p.PID != self && !strings.HasPrefix(p.Name, "WebKit") {
			continue
		}
		libs, err := gpuLibraries(p.PID)
		if err != nil {
			continue // the process ended while the tree was read
		}
		out = append(out, processGPU{PID: p.PID, Name: p.Name, Libraries: libs})
	}
	return out
}

func gpuLibraries(pid int) ([]string, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/maps", pid))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	seen := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 6 {
			continue // an anonymous mapping
		}
		if name := filepath.Base(fields[5]); gpuLibrary.MatchString(name) {
			seen[name] = true
		}
	}
	libs := make([]string, 0, len(seen))
	for name := range seen {
		libs = append(libs, name)
	}
	slices.Sort(libs)
	return libs, scanner.Err()
}
