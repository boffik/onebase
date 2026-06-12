package ui

import (
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
)

func (s *Server) scheduledList(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		s.renderForbidden(w, r)
		return
	}
	jobs := s.sched.Jobs()
	var runs []map[string]any
	for _, j := range jobs {
		last, _ := s.sched.Runs(r.Context(), j.Name, 1)
		row := map[string]any{
			"Job": j,
		}
		if len(last) > 0 {
			row["LastRun"] = last[0]
		}
		runs = append(runs, row)
	}
	s.render(w, r, "page-scheduled-list", map[string]any{
		"JobRows": runs,
	})
}

func (s *Server) scheduledDetail(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		s.renderForbidden(w, r)
		return
	}
	name := chi.URLParam(r, "name")
	if dec, err := url.PathUnescape(name); err == nil {
		name = dec
	}
	job := s.sched.GetJob(name)
	if job == nil {
		http.Error(w, "job not found: "+name, 404)
		return
	}
	runs, _ := s.sched.Runs(r.Context(), job.Name, 50)
	s.render(w, r, "page-scheduled-detail", map[string]any{
		"Job":  job,
		"Runs": runs,
	})
}

func (s *Server) scheduledRunNow(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		s.renderForbidden(w, r)
		return
	}
	name := chi.URLParam(r, "name")
	if dec, err := url.PathUnescape(name); err == nil {
		name = dec
	}
	if err := s.sched.RunNow(r.Context(), name); err != nil {
		http.Error(w, s.errText(r, err), 400)
		return
	}
	http.Redirect(w, r, "/ui/admin/scheduled/"+url.PathEscape(name), http.StatusSeeOther)
}
