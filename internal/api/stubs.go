package api

import "net/http"

// Stubs return 501 until their phase lands. They keep the router wired without
// blocking other work.

func notImplemented(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, http.StatusNotImplemented, "not_implemented", name+" not implemented yet")
	}
}

func listTemplatesHandler(d Deps) http.HandlerFunc  { return notImplemented("listTemplates") }
func installTemplateHandler(d Deps) http.HandlerFunc {
	return notImplemented("installTemplate")
}
func getSettingsHandler(d Deps) http.HandlerFunc { return notImplemented("getSettings") }
func putSettingsHandler(d Deps) http.HandlerFunc { return notImplemented("putSettings") }
func backupHandler(d Deps) http.HandlerFunc      { return notImplemented("backup") }
func restoreHandler(d Deps) http.HandlerFunc     { return notImplemented("restore") }
