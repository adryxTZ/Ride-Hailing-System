package trips

import "testing"

func TestPartyColumnOnlyKnownRoles(t *testing.T) {
	for role, want := range map[string]string{AsRider: "rider_id", AsDriver: "driver_id"} {
		if got, ok := partyColumn(role); !ok || got != want {
			t.Fatalf("partyColumn(%q) = %q, %v", role, got, ok)
		}
	}
	for _, bad := range []string{"", "rider", "id; DROP TABLE trips", "STAFF"} {
		if _, ok := partyColumn(bad); ok {
			t.Fatalf("partyColumn(%q) accepted", bad)
		}
	}
}

func TestValidIDs(t *testing.T) {
	if !validIDs("", "") || !validIDs("6bcf5c7c-f9c1-4bda-97a8-29cf6f81bb24", "") {
		t.Fatal("empty or UUID filters must pass")
	}
	if validIDs("not-a-uuid", "") || validIDs("", "1 OR 1=1") {
		t.Fatal("non-UUID filters must match nothing")
	}
}
