package api

import (
	"net/http"
	"testing"

	"github.com/rs/zerolog"
)

// The Control Center's write routes sit beside the existing admin routes of identity-service without a conflict
// (a conflicting pattern makes http.ServeMux panic at start-up).
func TestAdminManageRoutesDoNotConflict(t *testing.T) {
	mux := http.NewServeMux()
	for _, p := range []string{
		"GET /api/v1/admin/users", "POST /api/v1/admin/users/{user_id}/suspend", "POST /api/v1/admin/users/{user_id}/reactivate",
		"GET /api/v1/admin/monetization", "GET /api/v1/admin/supplier-applications", "PUT /api/v1/admin/exchange-rate",
	} {
		mux.HandleFunc(p, func(http.ResponseWriter, *http.Request) {})
	}
	NewAdminManageHandler(nil, zerolog.Nop()).Register(mux)
}
