package budgettui

import "testing"

func TestParseAmount(t *testing.T) {
	if n, err := parseAmount(" 50 "); err != nil || n != 50 {
		t.Fatalf("got (%d, %v), want (50, nil)", n, err)
	}
	for _, bad := range []string{"", "0", "-3", "abc", "1.5"} {
		if _, err := parseAmount(bad); err == nil {
			t.Errorf("parseAmount(%q) = nil error, want error", bad)
		}
	}
}

func TestToggleSKU(t *testing.T) {
	if got := toggleSKU(SKUAICredits); got != SKUPremiumRequests {
		t.Errorf("toggle ai_credits = %q, want %q", got, SKUPremiumRequests)
	}
	if got := toggleSKU(SKUPremiumRequests); got != SKUAICredits {
		t.Errorf("toggle premium_requests = %q, want %q", got, SKUAICredits)
	}
}

func TestFilterRows(t *testing.T) {
	all := []Row{{User: "alice"}, {User: "bob"}, {User: "ALBERT"}}
	got := filterRows(all, "al")
	if len(got) != 2 {
		t.Fatalf("filter 'al' returned %d rows, want 2", len(got))
	}
	if len(filterRows(all, "")) != 3 {
		t.Error("empty filter should return all rows")
	}
}

func TestCells(t *testing.T) {
	if budgetCell(nil) != "-" {
		t.Error("nil amount should render '-'")
	}
	n := 50
	if budgetCell(&n) != "$50" {
		t.Error("amount should render '$50'")
	}
	if consumedCell(nil) != "-" {
		t.Error("nil consumed should render '-'")
	}
	c := 12.4
	if consumedCell(&c) != "$12.40" {
		t.Error("consumed should render '$12.40'")
	}
}
