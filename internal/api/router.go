package api

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/flakerimi/basepod/internal/apps"
	"github.com/flakerimi/basepod/internal/auth"
	"github.com/flakerimi/basepod/internal/builder"
	"github.com/flakerimi/basepod/internal/caddy"
	"github.com/flakerimi/basepod/internal/config"
	bpcrypto "github.com/flakerimi/basepod/internal/crypto"
	"github.com/flakerimi/basepod/internal/deploy"
	"github.com/flakerimi/basepod/internal/events"
	"github.com/flakerimi/basepod/internal/podman"
	"github.com/flakerimi/basepod/internal/store/db"
)

type Deps struct {
	Cfg          config.Config
	DB           *sql.DB
	Queries      *db.Queries
	Auth         *auth.Service
	Apps         *apps.Service
	Builder      *builder.Builder
	Orchestrator *deploy.Orchestrator
	Locks        *deploy.AppLocks
	Podman       *podman.Client
	Caddy        *caddy.Client
	Events       *events.Hub
	EnvCipher    *bpcrypto.EnvCipher
	Log          *slog.Logger
	Version      string
}

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(bodyLimit)

	r.Get("/api/v1/health", healthHandler(d))
	r.Get("/api/v1/caddy/check", caddyCheckHandler(d))

	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Get("/status", authStatusHandler(d))
		r.With(loginRateLimit).Post("/setup", setupHandler(d))
		r.With(loginRateLimit).Post("/login", loginHandler(d))
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
		r.Post("/{name}/start", startAppHandler(d))
		r.Post("/{name}/stop", stopAppHandler(d))
		r.Post("/{name}/restart", restartAppHandler(d))
		r.Post("/{name}/rollback", rollbackHandler(d))
		r.Get("/{name}/logs", logsSSEHandler(d))
		r.Get("/{name}/env", getEnvHandler(d))
		r.Put("/{name}/env", putEnvHandler(d))
		r.Delete("/{name}/env/{key}", deleteEnvKeyHandler(d))
		r.Get("/{name}/versions", listVersionsHandler(d))
		r.Post("/{name}/domains", addDomainHandler(d))
		r.Delete("/{name}/domains/{domain}", removeDomainHandler(d))

		r.Get("/{name}/git", getAppGitHandler(d))
		r.Put("/{name}/git", putAppGitHandler(d))
		r.Post("/{name}/webhook-secret", rotateWebhookSecretHandler(d))

		r.Post("/{name}/ports", addPortHandler(d))
		r.Delete("/{name}/ports/{port}", deletePortHandler(d))
		r.Post("/{name}/volumes", addVolumeHandler(d))
		r.Delete("/{name}/volumes/{path}", deleteVolumeHandler(d))
	})

	// Webhook delivery — accepts a third-party POST without our auth header;
	// the body's HMAC signature is what authenticates it.
	r.Post("/api/v1/apps/{name}/webhook", webhookDeliveryHandler(d))

	r.With(d.Auth.Middleware).Get("/api/v1/templates", listTemplatesHandler(d))
	r.With(d.Auth.Middleware).Post("/api/v1/templates/install", installTemplateHandler(d))

	r.With(d.Auth.Middleware).Get("/api/v1/settings", getSettingsHandler(d))
	r.With(d.Auth.Middleware).Put("/api/v1/settings", putSettingsHandler(d))

	r.With(d.Auth.Middleware).Get("/api/v1/system/info", systemInfoHandler(d))
	r.With(d.Auth.Middleware).Get("/api/v1/system/storage", systemStorageHandler(d))
	r.With(d.Auth.Middleware).Get("/api/v1/system/processes", systemProcessesHandler(d))
	r.With(d.Auth.Middleware).Post("/api/v1/system/prune", systemPruneHandler(d))

	r.With(d.Auth.Middleware).Get("/api/v1/audit", auditLogHandler(d))

	r.With(d.Auth.Middleware).Get("/api/v1/events", eventsSSEHandler(d))
	r.With(d.Auth.Middleware).Post("/api/v1/backup", backupHandler(d))
	r.With(d.Auth.Middleware).Post("/api/v1/restore", restoreHandler(d))

	r.Mount("/", spaHandler(d))
	return r
}
