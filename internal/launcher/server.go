package launcher

import (
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server is the launcher HTTP server (list of registered bases).
type Server struct {
	h      *handler
	ln     net.Listener
	quit   chan struct{}
	httpSrv *http.Server
}

// NewServer creates a launcher server bound to a random available port.
func NewServer(store *Store, runner *Runner) (*Server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	h := &handler{store: store, runner: runner}
	return &Server{h: h, ln: ln, quit: make(chan struct{})}, nil
}

// URL returns the base URL of the launcher server.
func (s *Server) URL() string { return "http://" + s.ln.Addr().String() }

// Done returns a channel that is closed when /quit is received.
func (s *Server) Done() <-chan struct{} { return s.quit }

// Close shuts down the HTTP server, closes auth pools and kills any running
// base processes — otherwise onebase-gui.exe children survive as zombies when
// the launcher window is closed via the X button.
func (s *Server) Close() {
	if s.h != nil && s.h.runner != nil {
		var ports []int
		if s.h.store != nil {
			if bases, err := s.h.store.List(); err == nil {
				for _, b := range bases {
					ports = append(ports, b.Port)
				}
			}
		}
		s.h.runner.StopAll(ports)
	}
	CloseAuthPools()
	if s.httpSrv != nil {
		s.httpSrv.Close()
	}
}

func (s *Server) ListenAndServe() error {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	// Launcher pages (no auth)
	r.Get("/", s.h.index)
	r.Get("/bases/new", s.h.newForm)
	r.Post("/bases", s.h.create)
	r.Get("/bases/{id}/edit", s.h.editForm)
	r.Post("/bases/{id}", s.h.update)
	r.Post("/bases/{id}/delete", s.h.delete)
	r.Post("/bases/{id}/start", s.h.start)
	r.Post("/bases/{id}/stop", s.h.stop)
	r.Post("/bases/{id}/config/export", s.h.configExport)
	r.Post("/bases/{id}/config/import", s.h.configImport)

	// Configurator login (no auth — this IS the login page)
	r.Get("/bases/{id}/configurator/login", s.h.cfgLoginPage)
	r.Post("/bases/{id}/configurator/login", s.h.cfgLoginSubmit)

	// Configurator routes (auth required — admin only)
	r.Group(func(r chi.Router) {
		r.Use(s.h.cfgAuthMiddleware)
		r.Get("/bases/{id}/configurator", s.h.configuratorPage)
		r.Post("/bases/{id}/configurator/convert", s.h.configuratorConvert)
		r.Post("/bases/{id}/configurator/module", s.h.configuratorSaveModule)
		r.Post("/bases/{id}/configurator/fields", s.h.configuratorSaveFields)
		r.Post("/bases/{id}/configurator/form", s.h.configuratorSaveForm)
		r.Post("/bases/{id}/configurator/register-fields", s.h.configuratorSaveRegisterFields)
		r.Post("/bases/{id}/configurator/enum", s.h.configuratorSaveEnum)
		r.Post("/bases/{id}/configurator/constant", s.h.configuratorSaveConstant)
		r.Post("/bases/{id}/configurator/report", s.h.configuratorSaveReport)
		r.Post("/bases/{id}/configurator/common-module", s.h.configuratorSaveCommonModule)
		r.Post("/bases/{id}/configurator/processor", s.h.configuratorSaveProcessor)
		r.Post("/bases/{id}/configurator/new", s.h.configuratorNewObject)
		r.Post("/bases/{id}/configurator/printform", s.h.configuratorSavePrintForm)
		r.Post("/bases/{id}/configurator/layout", s.h.configuratorSaveLayout)
		r.Post("/bases/{id}/configurator/new-printform", s.h.configuratorNewPrintForm)
		r.Post("/bases/{id}/configurator/app", s.h.configuratorSaveApp)
		r.Post("/bases/{id}/configurator/subsystem", s.h.configuratorSaveSubsystem)
		r.Post("/bases/{id}/configurator/migrate", s.h.configuratorMigrate)
			r.Get("/bases/{id}/configurator/config/export-zip", s.h.configExportZip)
			r.Post("/bases/{id}/configurator/config/import-zip", s.h.configImportZip)
		r.Get("/bases/{id}/configurator/admin/users", s.h.cfgAdminUsers)
		r.Post("/bases/{id}/configurator/admin/users/create", s.h.cfgAdminUserCreate)
		r.Post("/bases/{id}/configurator/admin/users/delete", s.h.cfgAdminUserDelete)
		r.Get("/bases/{id}/configurator/admin/sessions", s.h.cfgAdminSessions)
		r.Post("/bases/{id}/configurator/admin/sessions/kick", s.h.cfgAdminSessionKick)
		r.Get("/bases/{id}/configurator/admin/audit", s.h.cfgAdminAudit)
		r.Get("/bases/{id}/configurator/admin/about", s.h.cfgAdminAbout)
		r.Post("/bases/{id}/configurator/backup/create", s.h.backupCreate)
		r.Get("/bases/{id}/configurator/backup/{file}/download", s.h.backupDownload)
		r.Post("/bases/{id}/configurator/backup/{file}/delete", s.h.backupDelete)
		r.Post("/bases/{id}/configurator/backup/settings", s.h.backupSettings)
			r.Post("/bases/{id}/configurator/backup/upload", s.h.backupUpload)
			r.Post("/bases/{id}/configurator/backup/{file}/restore", s.h.backupRestore)
			r.Get("/bases/{id}/configurator/backup/full-export", s.h.backupFullExport)
			r.Post("/bases/{id}/configurator/backup/full-import", s.h.backupFullImport)
		// Debug proxy — forwards /bases/{id}/debug/{action} to UI server (avoids CORS in webview)
		r.HandleFunc("/bases/{id}/debug/{action}", s.h.debugProxy) // GET + POST
	})

	r.Post("/killall", s.h.killAll)
	r.Post("/quit", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		close(s.quit)
	})

	s.httpSrv = &http.Server{Handler: r}
	return s.httpSrv.Serve(s.ln)
}
