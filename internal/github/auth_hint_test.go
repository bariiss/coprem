package github

import "testing"

func TestParseGHAuthAccounts(t *testing.T) {
	sample := `github.com
  ✓ Logged in to github.com account personal-account (keyring)
  - Active account: true
  - Token scopes: 'admin:org', 'repo'

  ✓ Logged in to github.com account enterprise-account (keyring)
  - Active account: false
  - Token scopes: 'admin:enterprise', 'repo'
`
	accounts, err := parseGHAuthAccountsFromOutput(sample, "github.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 {
		t.Fatalf("got %d accounts", len(accounts))
	}
	if !accounts[0].Active || accounts[0].HasEnterpriseScope {
		t.Fatalf("personal-account: %+v", accounts[0])
	}
	if accounts[1].Active || !accounts[1].HasEnterpriseScope {
		t.Fatalf("enterprise-account: %+v", accounts[1])
	}

	user, ok := preferredEnterpriseGHUserFromAccounts(accounts)
	if !ok || user != "enterprise-account" {
		t.Fatalf("preferred = %q ok=%v", user, ok)
	}
}
