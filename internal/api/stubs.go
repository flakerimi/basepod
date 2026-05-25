package api

import "net/http"

// Stubs return 501 until their phase lands. They keep the router wired without
// blocking other work.

func notImplemented(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, http.StatusNotImplemented, "not_implemented", name+" not implemented yet")
	}
}

func listAppsHandler(d Deps) http.HandlerFunc       { return notImplemented("listApps") }
func createAppHandler(d Deps) http.HandlerFunc      { return notImplemented("createApp") }
func getAppHandler(d Deps) http.HandlerFunc         { return notImplemented("getApp") }
func updateAppHandler(d Deps) http.HandlerFunc      { return notImplemented("updateApp") }
func deleteAppHandler(d Deps) http.HandlerFunc      { return notImplemented("deleteApp") }
func deployHandler(d Deps) http.HandlerFunc         { return notImplemented("deploy") }
func restartAppHandler(d Deps) http.HandlerFunc     { return notImplemented("restart") }
func rollbackHandler(d Deps) http.HandlerFunc       { return notImplemented("rollback") }
func logsSSEHandler(d Deps) http.HandlerFunc        { return notImplemented("logs") }
func getEnvHandler(d Deps) http.HandlerFunc         { return notImplemented("getEnv") }
func putEnvHandler(d Deps) http.HandlerFunc         { return notImplemented("putEnv") }
func listVersionsHandler(d Deps) http.HandlerFunc   { return notImplemented("listVersions") }
func addDomainHandler(d Deps) http.HandlerFunc      { return notImplemented("addDomain") }
func removeDomainHandler(d Deps) http.HandlerFunc   { return notImplemented("removeDomain") }
func listTemplatesHandler(d Deps) http.HandlerFunc  { return notImplemented("listTemplates") }
func installTemplateHandler(d Deps) http.HandlerFunc {
	return notImplemented("installTemplate")
}
func getSettingsHandler(d Deps) http.HandlerFunc { return notImplemented("getSettings") }
func putSettingsHandler(d Deps) http.HandlerFunc { return notImplemented("putSettings") }
func backupHandler(d Deps) http.HandlerFunc      { return notImplemented("backup") }
func restoreHandler(d Deps) http.HandlerFunc     { return notImplemented("restore") }
