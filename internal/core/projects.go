package core

import "net/http"

// The Project API. A folder in a request is an absolute path: the core's
// working folder is not the client's.
type (
	addProjectRequest struct {
		Dir  string `json:"dir"`
		Name string `json:"name,omitempty"` // empty: the main checkout's folder name
	}
	importRequest struct {
		Dir string `json:"dir"`
	}
)

func (s *Server) projectRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/projects", func(w http.ResponseWriter, r *http.Request) {
		o, err := s.projects.Overview()
		s.reply(w, err, o)
	})
	mux.HandleFunc("POST /api/projects", func(w http.ResponseWriter, r *http.Request) {
		var req addProjectRequest
		if decode(w, r, &req) {
			p, err := s.projects.Add(req.Dir, req.Name)
			s.reply(w, err, p)
		}
	})
	mux.HandleFunc("GET /api/projects/{name}", func(w http.ResponseWriter, r *http.Request) {
		d, err := s.projects.Get(r.PathValue("name"))
		s.reply(w, err, d)
	})
	mux.HandleFunc("DELETE /api/projects/{name}", func(w http.ResponseWriter, r *http.Request) {
		s.reply(w, s.projects.Remove(r.PathValue("name")), nil)
	})
	mux.HandleFunc("POST /api/projects/import", func(w http.ResponseWriter, r *http.Request) {
		var req importRequest
		if decode(w, r, &req) {
			rep, err := s.projects.Import(req.Dir)
			s.reply(w, err, rep)
		}
	})
}
