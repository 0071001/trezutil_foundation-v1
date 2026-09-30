package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/trezutil/trezutil/internal/application/vertical"
)

func TestListVerticalsH(t *testing.T) {
	w := httptest.NewRecorder()
	ListVerticalsH(vertical.DefaultCatalog())(w, httptest.NewRequest(http.MethodGet, "/v1/verticals", nil))
	var resp struct{ Items []VerticalDTO }
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || w.Code != http.StatusOK {
		t.Fatalf("%d %v", w.Code, err)
	}
	byCode := map[string]VerticalDTO{}
	for _, v := range resp.Items {
		byCode[v.Code] = v
	}
	if len(resp.Items) != 10 {
		t.Fatalf("verticals = %d", len(resp.Items))
	}
	for code, flow := range map[string]string{"HOTEL": "BOOK", "RETAIL": "BUY", "SERVICES": "BOOK", "RESTAURANT": "BOOK"} {
		if v := byCode[code]; len(v.Flows) == 0 || v.Flows[0] != flow {
			t.Errorf("%s flows = %v, want %s first", code, v.Flows, flow)
		}
	}
	if r := byCode["RESTAURANT"]; len(r.Flows) != 2 || r.Flows[1] != "BUY" {
		t.Errorf("restaurants book tables and take orders: %v", r.Flows)
	}
	if w.Header().Get("Cache-Control") == "" {
		t.Error("the catalogue should be cacheable")
	}
}
