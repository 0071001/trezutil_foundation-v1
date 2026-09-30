package integration

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/domain"
)

// Store and organization addresses are read back (they feed the hotel
// property page, "Sold by … city" and place search).
func TestLocationFieldsRoundTrip(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)
	ctx := context.Background()
	const org, store = "10ca7100-0000-4000-8000-00000000000a", "10ca7100-0000-4000-8000-0000000007a1"
	province, city, address := "Kinshasa", "Kinshasa", "12 Avenue du Fleuve, Gombe"

	o := domain.NewOrganization(domain.OrganizationTypeBusiness, "Fleuve SARL", "Hotel du Fleuve", "CD", "+243810000900")
	o.ID, o.Province, o.City, o.Address = org, &province, &city, &address
	orgs := postgres.NewOrganizationRepository(db.DB)
	if err := orgs.Create(ctx, o); err != nil {
		t.Fatal(err)
	}
	s := domain.NewStore(org, "Hotel du Fleuve", "hotel-du-fleuve", "CD", "+243810000900")
	s.ID, s.Province, s.City, s.Address = store, &province, &city, &address
	stores := postgres.NewStoreRepository(db.DB)
	if err := stores.Create(ctx, s); err != nil {
		t.Fatal(err)
	}

	gotOrg, err := orgs.FindByID(ctx, org)
	if err != nil || gotOrg.City == nil || *gotOrg.City != city || gotOrg.Address == nil || *gotOrg.Address != address || gotOrg.Province == nil {
		t.Fatalf("organization location = %+v %v", gotOrg, err)
	}
	gotStore, err := stores.FindByID(ctx, store)
	if err != nil || gotStore.City == nil || *gotStore.City != city || gotStore.Address == nil || *gotStore.Address != address {
		t.Fatalf("store location = %+v %v", gotStore, err)
	}
	list, err := stores.FindByOrganizationID(ctx, org)
	if err != nil || len(list) != 1 || list[0].City == nil || *list[0].Address != address {
		t.Fatalf("store list location = %+v %v", list, err)
	}
}
