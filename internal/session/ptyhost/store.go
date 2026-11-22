package ptyhost

import (
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/stevetosak/booster/internal/jsonfile"
	"github.com/stevetosak/booster/internal/session"
)

// stateVersion is the version of the session list this booster reads and
// writes.
const stateVersion = 1

// stateFile is the session list the host keeps for the next core.
type stateFile struct {
	Version  int      `json:"version"`
	Sessions []record `json:"sessions"`
}

// record is what the host keeps of a session to start it again after a
// core restart.
type record struct {
	ID      string       `json:"id"`
	Kind    session.Kind `json:"kind"`
	Dir     string       `json:"dir"`
	Agent   string       `json:"agent,omitempty"`
	Size    session.Size `json:"size"`
	Started time.Time    `json:"started"`
}

func recordOf(i session.Info) record {
	return record{ID: i.ID, Kind: i.Kind, Dir: i.Dir, Agent: i.Agent, Size: i.Size, Started: i.Started}
}

func (r record) info() session.Info {
	return session.Info{ID: r.ID, Kind: r.Kind, Dir: r.Dir, Agent: r.Agent, Size: r.Size, Started: r.Started}
}

// save writes the live sessions to the state file. While the host shuts down
// the file keeps the list as it was, for the next core.
func (h *Host) save() {
	if h.opts.StateFile == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stopping {
		return
	}
	list := make([]record, 0, len(h.sessions))
	for _, s := range h.sessions {
		list = append(list, recordOf(s.snapshotInfo()))
	}
	slices.SortFunc(list, func(a, b record) int { return a.Started.Compare(b.Started) })
	if err := jsonfile.Write(h.opts.StateFile, stateFile{Version: stateVersion, Sessions: list}); err != nil {
		log.Printf("save the session list: %v", err)
	}
}

// load reads the session list the last core left.
func (h *Host) load() ([]record, error) {
	if h.opts.StateFile == "" {
		return nil, nil
	}
	var f stateFile
	if ok, err := jsonfile.Read(h.opts.StateFile, &f); err != nil || !ok {
		return nil, err
	}
	if f.Version != stateVersion {
		return nil, fmt.Errorf("%s: version %d, this booster reads version %d", h.opts.StateFile, f.Version, stateVersion)
	}
	return f.Sessions, nil
}
