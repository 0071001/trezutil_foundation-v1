package api

import (
	"net/http"

	"github.com/trezutil/trezutil/packages/auth"
)

// asQueryUser simulates an authenticated request for the actor named in the
// test URL's user_id parameter. Handlers ignore the parameter itself; identity
// only ever comes from the verified principal in the request context.
func asQueryUser(r *http.Request) *http.Request {
	if id := r.URL.Query().Get("user_id"); id != "" {
		return r.WithContext(auth.WithPrincipal(r.Context(), auth.Principal{UserID: id}))
	}
	return r
}
