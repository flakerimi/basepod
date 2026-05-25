package api

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/flakerimi/basepod/internal/auth"
	"github.com/flakerimi/basepod/internal/caddy"
	"github.com/flakerimi/basepod/internal/config"
	"github.com/flakerimi/basepod/internal/events"
	"github.com/flakerimi/basepod/internal/podman"
	"github.com/flakerimi/basepod/internal/store/db"
)

type Deps struct {
	Cfg     config.Config
	DB      *sql.DB
	Queries *db.Queries
	Auth    *auth.Service
	Podman  *podman.Client
	Caddy   *caddy.Client
	Events  *events.Hub
	Log     *slog.Logger
	Version string
}

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	r.Get("/api/v1/health", healthHandler(d))

	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/login", loginHandler(d))
		r.With(d.Auth.Middleware).Post("/logout", logoutHandler(d))
		r.With(d.Auth.Middleware).Get("/me", meHandler(d))
		r.With(d.Auth.Middleware).Get("/tokens", listTokensHandler(d))
		r.With(d.Auth.Middleware).Post("/tokens", createTokenHandler(d))
		r.With(d.Auth.Middleware).Delete("/tokens/{id}", revokeTokenHandler(d))
	})

	r.With(d.Auth.Middleware).Route("/api/v1/apps", func(r chi.Router) {
		r.Get("/", listAppsHandler(d))
		r.Post("/", createAppHandler(d))
		r.Get("/{name}", getAppHandler(d))
		r.Patch("/{name}", updateAppHandler(d))
		r.Delete("/{name}", deleteAppHandler(d))
		r.Post("/{name}/deploy", deployHandler(d))
		r.Post("/{name}/restart", restartAppHandler(d))
		r.Post("/{name}/rollback", rollbackHandler(d))
		r.Get("/{name}/logs", logsSSEHandler(d))
		r.Get("/{name}/env", getEnvHandler(d))
		r.Put("/{name}/env", putEnvHandler(d))
		r.Get("/{name}/versions", listVersionsHandler(d))
		r.Post("/{name}/domains", addDomainHandler(d))
		r.Delete("/{name}/domains/{domain}", removeDomainHandler(d))
	})

	r.With(d.Auth.Middleware).Get("/api/v1/templates", listTemplatesHandler(d))
	r.With(d.Auth.Middleware).Post("/api/v1/templates/install", installTemplateHandler(d))

	r.With(d.Auth.Middleware).Get("/api/v1/settings", getSettingsHandler(d))
	r.With(d.Auth.Middleware).Put("/api/v1/settings", putSettingsHandler(d))

	r.With(d.Auth.Middleware).Get("/api/v1/events", eventsSSEHandler(d))
	r.With(d.Auth.Middleware).Post("/api/v1/backup", backupHandler(d))
	r.With(d.Auth.Middleware).Post("/api/v1/restore", restoreHandler(d))

	r.Mount("/", spaHandler(d))
	return r
}
