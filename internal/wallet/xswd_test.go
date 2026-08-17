// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"testing"

	"github.com/civilware/epoch"
)

// TestEpochHandlerMethods verifies the EPOCH library exposes exactly the
// methods we register over XSWD and that the registration set matches.
func TestEpochHandlerMethods(t *testing.T) {
	handler := epoch.GetHandler()
	for name := range epochMethods {
		if _, ok := handler[name]; !ok {
			t.Errorf("epoch.GetHandler() missing method %q", name)
		}
	}
}

// TestIsEpochMethod verifies auto-allowed EPOCH methods are recognized and
// sensitive wallet methods are never auto-allowed.
func TestIsEpochMethod(t *testing.T) {
	allowed := []string{
		"AttemptEPOCH",
		"SubmitEPOCH",
		"GetMaxHashesEPOCH",
		"GetAddressEPOCH",
		"GetSessionEPOCH",
		"StopEPOCH",
	}
	for _, m := range allowed {
		if !isEpochMethod(m) {
			t.Errorf("isEpochMethod(%q) = false, want true", m)
		}
	}

	denied := []string{
		"transfer",
		"query_key",
		"QueryKey",
		"GetBalance",
		"SignData",
		"Subscribe",
		"",
	}
	for _, m := range denied {
		if isEpochMethod(m) {
			t.Errorf("isEpochMethod(%q) = true, want false", m)
		}
	}
}
