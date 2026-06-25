package github

import (
	"os"
	"testing"
)

func TestPreferredEnterpriseGHUserIntegration(t *testing.T) {
	if os.Getenv("COPREM_INTEGRATION") == "" {
		t.Skip("set COPREM_INTEGRATION=1 to run")
	}
	accounts, err := parseGHAuthAccounts("github.com")
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range accounts {
		t.Logf("account=%+v", account)
	}
	user, ok := preferredEnterpriseGHUserFromAccounts(accounts)
	t.Logf("preferred=%q ok=%v", user, ok)
}
