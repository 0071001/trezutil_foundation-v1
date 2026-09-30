package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestParseStay(t *testing.T) {
	s, err := ParseStay("2026-10-10", "2026-10-12")
	if err != nil || s.Nights() != 2 || s.StartAt() != 1791590400 || s.EndAt() != 1791590400+2*86400 {
		t.Fatalf("stay = %+v %v", s, err)
	}
	for name, dates := range map[string][2]string{
		"same day":    {"2026-10-10", "2026-10-10"},
		"backwards":   {"2026-10-12", "2026-10-10"},
		"not a date":  {"10/10/2026", "2026-10-12"},
		"too long":    {"2026-10-01", "2026-11-15"},
		"missing out": {"2026-10-10", ""},
		"datetime":    {"2026-10-10T14:00", "2026-10-12"},
	} {
		if _, err := ParseStay(dates[0], dates[1]); !errors.Is(err, ErrInvalidParameter) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := NewStay(1791590400+14*3600, 1791590400+2*86400); !errors.Is(err, ErrInvalidParameter) {
		t.Error("a stay must start at a night boundary")
	}
	if NightDate(1791590400) != "2026-10-10" || !IsNightBoundary(1791590400) || IsNightBoundary(1791590401) {
		t.Error("night helpers")
	}
}

func TestHotelRoomType_Rules(t *testing.T) {
	rt := &HotelRoomType{MaxOccupancy: 2, BedConfiguration: "  1 king bed ", Amenities: []string{" Wi-Fi", "wi-fi", "", "Pool"}}
	rt.Normalize()
	if rt.BedConfiguration != "1 king bed" || len(rt.Amenities) != 2 || rt.Amenities[0] != "Wi-Fi" {
		t.Fatalf("normalized = %+v", rt)
	}
	if err := rt.Validate(); err != nil {
		t.Fatal(err)
	}
	if rt.RoomsNeeded(1) != 1 || rt.RoomsNeeded(2) != 1 || rt.RoomsNeeded(3) != 2 || rt.RoomsNeeded(0) != 1 {
		t.Error("rooms needed")
	}
	for name, bad := range map[string]HotelRoomType{
		"no occupancy":   {MaxOccupancy: 0},
		"too many":       {MaxOccupancy: MaxHotelOccupancy + 1},
		"long bed":       {MaxOccupancy: 1, BedConfiguration: strings.Repeat("b", 121)},
		"long amenity":   {MaxOccupancy: 1, Amenities: []string{strings.Repeat("a", 61)}},
		"many amenities": {MaxOccupancy: 1, Amenities: make([]string, 31)},
	} {
		b := bad
		if err := b.Validate(); !errors.Is(err, ErrInvalidParameter) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestCapacityCoverage_TightestWindowWithoutGaps(t *testing.T) {
	night := func(i int64, capacity, reserved int64) *CapacityUnit {
		return &CapacityUnit{StartAt: 1791590400 + i*86400, EndAt: 1791590400 + (i+1)*86400, Capacity: capacity, Reserved: reserved}
	}
	start := int64(1791590400)
	units := []*CapacityUnit{night(0, 10, 0), night(1, 8, 2), night(2, 6, 0)}
	if avail, capacity, ok := CapacityCoverage(units, start, start+3*86400); !ok || avail != 6 || capacity != 6 {
		t.Fatalf("three nights = %d/%d %v", avail, capacity, ok)
	}
	if avail, _, ok := CapacityCoverage(units, start+86400, start+2*86400); !ok || avail != 6 {
		t.Fatalf("night 1 = %d %v", avail, ok) // 8 offered, 2 booked
	}
	gap := []*CapacityUnit{night(0, 10, 0), night(2, 6, 0)}
	if _, _, ok := CapacityCoverage(gap, start, start+3*86400); ok {
		t.Fatal("a missing night is not covered")
	}
	if _, _, ok := CapacityCoverage(units, start, start+4*86400); ok {
		t.Fatal("a period past the last window is not covered")
	}
	if _, _, ok := CapacityCoverage(nil, start, start+86400); ok {
		t.Fatal("no windows, no coverage")
	}
	if !ResourceTypeRoom.IsTimeWindowed() || ResourceTypeCountable.IsTimeWindowed() {
		t.Fatal("rooms are booked by windows, stock is counted")
	}
}
