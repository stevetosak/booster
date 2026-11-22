package ptyhost

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"

	"booster/spikes/session/session"
)

// Recorder keeps a session's raw output and size changes, so the screen
// restore can be compared with xterm.js on the same bytes (`session replay`
// and compare/compare.mjs).
type Recorder interface {
	Output(chunk []byte)
	Resize(session.Size)
	Close()
}

type nopRecorder struct{}

func (nopRecorder) Output([]byte)       {}
func (nopRecorder) Resize(session.Size) {}
func (nopRecorder) Close()              {}

// Recording describes one recorded session: <id>.raw holds the output bytes,
// <id>.json this description.
type Recording struct {
	Kind    session.Kind `json:"kind"`
	Size    session.Size `json:"size"`
	Resizes []Resize     `json:"resizes"`
}

// Resize is a size change after Offset output bytes.
type Resize struct {
	Offset int64        `json:"offset"`
	Size   session.Size `json:"size"`
}

// FileRecorder records into a folder.
type FileRecorder struct {
	mu     sync.Mutex
	raw    *os.File
	meta   string
	rec    Recording
	offset int64
}

// RecordTo returns an Options.Record that writes each session into dir.
func RecordTo(dir string) func(session.Info) (Recorder, error) {
	return func(info session.Info) (Recorder, error) {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
		raw, err := os.OpenFile(filepath.Join(dir, info.ID+".raw"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return nil, err
		}
		r := &FileRecorder{raw: raw, meta: filepath.Join(dir, info.ID+".json"), rec: Recording{Kind: info.Kind, Size: info.Size}}
		r.writeMeta()
		return r, nil
	}
}

func (r *FileRecorder) Output(chunk []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n, _ := r.raw.Write(chunk)
	r.offset += int64(n)
}

func (r *FileRecorder) Resize(size session.Size) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rec.Resizes = append(r.rec.Resizes, Resize{Offset: r.offset, Size: size})
	r.writeMeta()
}

func (r *FileRecorder) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	_ = r.raw.Close()
}

func (r *FileRecorder) writeMeta() {
	b, _ := json.MarshalIndent(r.rec, "", "  ")
	_ = os.WriteFile(r.meta, b, 0o600)
}

// Replay feeds a recording's output into a new emulator, with its size
// changes, and returns the snapshot a client would get after it.
func Replay(raw []byte, rec Recording, scrollback int) []byte {
	scr := newScreen(int(rec.Size.Cols), int(rec.Size.Rows), scrollback)
	// The emulator writes replies to queries into a pipe that blocks until read.
	go func() { _, _ = io.Copy(io.Discard, scr.emu) }()
	var at int64
	for _, rs := range rec.Resizes {
		end := min(rs.Offset, int64(len(raw)))
		scr.write(raw[at:end])
		scr.emu.Resize(int(rs.Size.Cols), int(rs.Size.Rows))
		at = end
	}
	scr.write(raw[at:])
	return scr.snapshot()
}
