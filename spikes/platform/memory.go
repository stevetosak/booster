package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// processMemory is the proportional set size of one process: shared pages
// count once across the processes that share them.
type processMemory struct {
	PID   int    `json:"pid"`
	Name  string `json:"name"`
	PssKB int64  `json:"pssKB"`
}

// memoryReport splits this process tree into the shell (this process and
// WebKit's helpers) and the programs that run in the terminals.
type memoryReport struct {
	ShellKB    int64           `json:"shellKB"`
	ProgramsKB int64           `json:"programsKB"`
	Processes  []processMemory `json:"processes"`
}

func readMemory() (memoryReport, error) {
	var r memoryReport
	children, err := childrenByParent()
	if err != nil {
		return r, err
	}
	self := os.Getpid()
	queue := []int{self}
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		queue = append(queue, children[pid]...)

		name := readComm(pid)
		pss, err := readPss(pid)
		if err != nil {
			continue // the process ended while the tree was read
		}
		r.Processes = append(r.Processes, processMemory{PID: pid, Name: name, PssKB: pss})
		if pid == self || strings.HasPrefix(name, "WebKit") {
			r.ShellKB += pss
		} else {
			r.ProgramsKB += pss
		}
	}
	return r, nil
}

func childrenByParent() (map[int][]int, error) {
	stats, err := filepath.Glob("/proc/[0-9]*/stat")
	if err != nil {
		return nil, err
	}
	children := make(map[int][]int)
	for _, path := range stats {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		pid, ppid, ok := parseStat(data)
		if ok {
			children[ppid] = append(children[ppid], pid)
		}
	}
	return children, nil
}

// parseStat reads the pid and the parent pid from /proc/<pid>/stat. The
// command name sits in parentheses and may hold spaces, so the fields after
// it are split from the last ')'.
func parseStat(data []byte) (pid, ppid int, ok bool) {
	end := bytes.LastIndexByte(data, ')')
	open := bytes.IndexByte(data, '(')
	if end < 0 || open < 1 {
		return 0, 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data[:open])))
	if err != nil {
		return 0, 0, false
	}
	fields := strings.Fields(string(data[end+1:]))
	if len(fields) < 2 {
		return 0, 0, false
	}
	ppid, err = strconv.Atoi(fields[1])
	return pid, ppid, err == nil
}

func readComm(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return "?"
	}
	return strings.TrimSpace(string(data))
}

func readPss(pid int) (int64, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/smaps_rollup", pid))
	if err != nil {
		return 0, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "Pss:" {
			return strconv.ParseInt(fields[1], 10, 64)
		}
	}
	return 0, fmt.Errorf("no Pss line for pid %d", pid)
}
