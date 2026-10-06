package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

/*
 * Findings S-1 and S-2: a guest could read any seller's phone, email and account
 * id, and any store's phone, email and organization id, from an id found in a
 * public search result. These tests pin who reads what.
 */

var errNoSuchProfile = errors.New("not found")

type profileSellers struct{ mockSellerUseCases }

func (*profileSellers) GetSellerProfile(_ context.Context, id string) (*domain.SellerProfile, error) {
	if id != "sp1" {
		return nil, errNoSuchProfile
	}
	user, org, email, bio := "u-seller", "org1", "owner@gombe.example", "Hotel in Gombe"
	return &domain.SellerProfile{
		ID: "sp1", UserID: &user, OrganizationID: &org, SellerType: "business", DisplayName: "Gombe Stays", Bio: &bio,
		Phone: "+243810000001", Email: &email, Status: "active", VerificationStatus: "verified", TrustLevel: 2, CreatedAt: 100,
	}, nil
}

type profileStores struct{ mockStoreUseCases }

func (*profileStores) GetStore(_ context.Context, id string) (*domain.Store, error) {
	if id != "s1" {
		return nil, errNoSuchProfile
	}
	email, city, addr := "contact@gombe.example", "Kinshasa", "Av. de la Gombe 12"
	lat, lng := -4.30, 15.30
	return &domain.Store{
		ID: "s1", OrganizationID: "org1", Name: "Gombe Stays", Slug: "gombe-stays", Phone: "+243810000002", Email: &email,
		Country: "CD", City: &city, Address: &addr, Latitude: &lat, Longitude: &lng, Status: "active",
	}, nil
}

type fakeProfileAccess struct {
	staff   map[string]bool
	members map[string]string // user -> organization
}

func (f fakeProfileAccess) IsPlatformStaff(_ context.Context, userID string) bool {
	return f.staff[userID]
}
func (f fakeProfileAccess) IsMember(_ context.Context, org, userID string) bool {
	return org != "" && f.members[userID] == org
}

var access = fakeProfileAccess{
	staff:   map[string]bool{"u-operator": true, "u-admin": true},
	members: map[string]string{"u-owner": "org1", "u-other-owner": "org2"},
}

func fetch(t *testing.T, h http.HandlerFunc, url, user string) (int, map[string]any) {
	t.Helper()
	if user != "" {
		url += "&user_id=" + user
	}
	rec := httptest.NewRecorder()
	h(rec, asQueryUser(httptest.NewRequest(http.MethodGet, url, nil)))
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}

var sellerSecrets = []string{"Phone", "Email", "UserID", "OrganizationID"}
var storeSecrets = []string{"Phone", "Email", "OrganizationID"}

func assertAbsent(t *testing.T, who string, body map[string]any, keys []string) {
	t.Helper()
	for _, k := range keys {
		if _, leaked := body[k]; leaked {
			t.Errorf("%s must not read %s", who, k)
		}
	}
}

func TestGetSeller_AGuestOrAStrangerReadsOnlyThePublicProjection(t *testing.T) {
	h := NewSellerHandler(&profileSellers{}).WithAccess(access)
	for _, who := range []string{"", "u-buyer", "u-other-owner"} { // a guest, any customer, the owner of another business
		name := who
		if name == "" {
			name = "a guest"
		}
		code, body := fetch(t, h.GetSellerH, "/api/v1/sellers?seller_id=sp1", who)
		if code != http.StatusOK {
			t.Fatalf("%s: status %d", name, code)
		}
		assertAbsent(t, name, body, sellerSecrets)
		if body["DisplayName"] != "Gombe Stays" || body["VerificationStatus"] != "verified" || body["TrustLevel"] != float64(2) {
			t.Errorf("%s still reads what a buyer needs (name, verification, trust): %v", name, body)
		}
	}
}

func TestGetSeller_TheSellerItsOrganizationAndPlatformStaffReadTheWholeProfile(t *testing.T) {
	h := NewSellerHandler(&profileSellers{}).WithAccess(access)
	for _, who := range []string{"u-seller", "u-owner", "u-operator", "u-admin"} {
		code, body := fetch(t, h.GetSellerH, "/api/v1/sellers?seller_id=sp1", who)
		if code != http.StatusOK || body["Phone"] != "+243810000001" || body["Email"] != "owner@gombe.example" || body["OrganizationID"] != "org1" || body["UserID"] != "u-seller" {
			t.Errorf("%s must read the whole profile: %d %v", who, code, body)
		}
	}
}

func TestGetSeller_WithNoAccessRulesNobodyReadsMoreThanThePublicProjection(t *testing.T) {
	h := NewSellerHandler(&profileSellers{}) // the safe default
	_, body := fetch(t, h.GetSellerH, "/api/v1/sellers?seller_id=sp1", "u-operator")
	assertAbsent(t, "anyone", body, sellerSecrets)
}

func TestGetSeller_AnUnknownSellerIs404ForEveryone(t *testing.T) {
	h := NewSellerHandler(&profileSellers{}).WithAccess(access)
	for _, who := range []string{"", "u-owner"} {
		if code, _ := fetch(t, h.GetSellerH, "/api/v1/sellers?seller_id=nope", who); code != http.StatusNotFound {
			t.Errorf("status %d, want 404", code)
		}
	}
}

func TestGetStore_AGuestOrAStrangerReadsOnlyThePublicProjection(t *testing.T) {
	h := NewStoreHandler(&profileStores{}).WithAccess(access)
	for _, who := range []string{"", "u-buyer", "u-other-owner"} {
		code, body := fetch(t, h.GetStoreH, "/api/v1/stores?store_id=s1", who)
		if code != http.StatusOK {
			t.Fatalf("status %d", code)
		}
		assertAbsent(t, "caller "+who, body, storeSecrets)
		if body["Name"] != "Gombe Stays" || body["City"] != "Kinshasa" || body["Address"] != "Av. de la Gombe 12" || body["Latitude"] != -4.30 {
			t.Errorf("a buyer still reads where the store is: %v", body)
		}
	}
}

func TestGetStore_MembersOfTheOrganizationAndPlatformStaffReadTheWholeRecord(t *testing.T) {
	h := NewStoreHandler(&profileStores{}).WithAccess(access)
	for _, who := range []string{"u-owner", "u-operator", "u-admin"} {
		code, body := fetch(t, h.GetStoreH, "/api/v1/stores?store_id=s1", who)
		if code != http.StatusOK || body["Phone"] != "+243810000002" || body["Email"] != "contact@gombe.example" || body["OrganizationID"] != "org1" {
			t.Errorf("%s must read the whole store record: %d %v", who, code, body)
		}
	}
}

func TestGetStore_WithNoAccessRulesNobodyReadsMoreThanThePublicProjection(t *testing.T) {
	h := NewStoreHandler(&profileStores{})
	_, body := fetch(t, h.GetStoreH, "/api/v1/stores?store_id=s1", "u-owner")
	assertAbsent(t, "anyone", body, storeSecrets)
}

func TestProfileProjections_NeverCarryAFieldTheFullRecordKeepsPrivate(t *testing.T) {
	// A field added to the full record is private until someone adds it to the public type on purpose.
	seller, _ := (&profileSellers{}).GetSellerProfile(context.Background(), "sp1")
	raw, _ := json.Marshal(publicSeller(seller))
	for _, secret := range []string{"+243810000001", "owner@gombe.example", "u-seller", "org1"} {
		if strings.Contains(string(raw), secret) {
			t.Errorf("the public seller carries %q: %s", secret, raw)
		}
	}
	store, _ := (&profileStores{}).GetStore(context.Background(), "s1")
	raw, _ = json.Marshal(publicStore(store))
	for _, secret := range []string{"+243810000002", "contact@gombe.example", "org1"} {
		if strings.Contains(string(raw), secret) {
			t.Errorf("the public store carries %q: %s", secret, raw)
		}
	}
}
