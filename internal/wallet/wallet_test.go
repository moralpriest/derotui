// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/deroproject/derohe/rpc"
)

func TestNormalizeDaemonAddress(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "host port", in: "localhost:10102", want: "localhost:10102"},
		{name: "http url", in: "http://localhost:10102", want: "http://localhost:10102"},
		{name: "https default port", in: "https://node.example.com", want: "https://node.example.com:443"},
		{name: "https custom port", in: "https://node.example.com:40402", want: "https://node.example.com:40402"},
		{name: "trim spaces", in: "  localhost:20000  ", want: "localhost:20000"},
		{name: "invalid path", in: "https://node.example.com:40402/json_rpc", wantErr: true},
		{name: "invalid no port", in: "localhost", wantErr: true},
		{name: "invalid scheme", in: "ftp://node.example.com", wantErr: true},
		{name: "empty", in: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeDaemonAddress(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (value=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeDaemonAddress(%q)=%q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestTxCacheThreadSafety tests concurrent access to the transaction cache
func TestTxCacheThreadSafety(t *testing.T) {
	// Create a mock wallet with cache fields
	w := &Wallet{
		// txCacheMu is zero value which is ready to use
	}

	var wg sync.WaitGroup

	// Concurrent cache writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			w.txCacheMu.Lock()
			w.txCache = []TransactionInfo{
				{TxID: "test", Amount: int64(n)},
			}
			w.txCacheHeight = uint64(n)
			w.txCacheTime = time.Now()
			w.txCacheMu.Unlock()
		}(i)
	}

	// Concurrent cache reads (simulating GetTransactions)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				w.txCacheMu.RLock()
				_ = w.txCacheHeight
				_ = w.txCacheTime
				// Make a copy to avoid race
				result := make([]TransactionInfo, len(w.txCache))
				copy(result, w.txCache)
				w.txCacheMu.RUnlock()
			}
		}()
	}

	// Concurrent cache invalidations (simulating after transfer)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				w.txCacheMu.Lock()
				w.txCache = nil
				w.txCacheHeight = 0
				w.txCacheTime = time.Time{}
				w.txCacheMu.Unlock()
			}
		}()
	}

	wg.Wait()
}

// TestTxCacheInvalidation tests that InvalidateTxCache properly clears the cache
func TestTxCacheInvalidation(t *testing.T) {
	w := &Wallet{}

	// Set some cache data
	w.txCacheMu.Lock()
	w.txCache = []TransactionInfo{{TxID: "test", Amount: 100}}
	w.txCacheHeight = 1000
	w.txCacheTime = time.Now()
	w.txCacheMu.Unlock()

	// Invalidate
	w.InvalidateTxCache()

	// Verify cleared
	w.txCacheMu.RLock()
	if w.txCache != nil {
		t.Error("Expected txCache to be nil after invalidation")
	}
	if w.txCacheHeight != 0 {
		t.Errorf("Expected txCacheHeight to be 0, got %d", w.txCacheHeight)
	}
	w.txCacheMu.RUnlock()
}

func TestShouldUseCachedTxsDuringSync(t *testing.T) {
	tests := []struct {
		name     string
		started  bool
		inFlight bool
		want     bool
	}{
		{name: "sync just started by this call", started: true, inFlight: true, want: false},
		{name: "sync just started with stale in-flight read", started: true, inFlight: false, want: false},
		{name: "separate sync already in flight", started: false, inFlight: true, want: true},
		{name: "no sync in flight", started: false, inFlight: false, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldUseCachedTxsDuringSync(tt.started, tt.inFlight)
			if got != tt.want {
				t.Fatalf("shouldUseCachedTxsDuringSync(%v, %v) = %v, want %v", tt.started, tt.inFlight, got, tt.want)
			}
		})
	}
}

// TestValidateSeed tests seed phrase validation
func TestValidateSeed(t *testing.T) {
	tests := []struct {
		name    string
		seed    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid seed - 25 abandon words",
			seed:    "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon",
			wantErr: false,
		},
		{
			name:    "empty seed",
			seed:    "",
			wantErr: true,
			errMsg:  "please enter your seed words",
		},
		{
			name:    "too few words - 10",
			seed:    "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon",
			wantErr: true,
			errMsg:  "not enough words",
		},
		{
			name:    "too many words - 26",
			seed:    "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon",
			wantErr: true,
			errMsg:  "too many words",
		},
		{
			name:    "invalid word",
			seed:    "abandon abandon abandon abandon xyznotvalid abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon",
			wantErr: true,
			errMsg:  "invalid word",
		},
		{
			name:    "multiple invalid words",
			seed:    "notaword abandon notaword2 abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon",
			wantErr: true,
			errMsg:  "invalid words",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSeed(tt.seed)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSeed() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("ValidateSeed() error message = %v, want containing %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// TestValidateWord tests single word validation
func TestValidateWord(t *testing.T) {
	tests := []struct {
		name    string
		word    string
		wantErr bool
	}{
		{
			name:    "valid word",
			word:    "abandon",
			wantErr: false,
		},
		{
			name:    "valid word uppercase",
			word:    "ABANDON",
			wantErr: false,
		},
		{
			name:    "valid word mixed case",
			word:    "AbAnDoN",
			wantErr: false,
		},
		{
			name:    "invalid word",
			word:    "notaword123",
			wantErr: true,
		},
		{
			name:    "empty word",
			word:    "",
			wantErr: false, // Empty is allowed (for placeholder)
		},
		{
			name:    "random garbage",
			word:    "xyzqwert",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWord(tt.word)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWord() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResolveTransferDestination_Address(t *testing.T) {
	addr := "dero1valid"
	parse := func(s string) (*rpc.Address, error) {
		if s == addr {
			return &rpc.Address{}, nil
		}
		return nil, fmt.Errorf("invalid")
	}
	resolved, _, err := resolveTransferDestination(addr, parse, func(name string) (string, error) {
		return "", fmt.Errorf("unexpected resolve call for %s", name)
	})
	if err != nil {
		t.Fatalf("expected address to pass directly, got error: %v", err)
	}
	if resolved != addr {
		t.Fatalf("resolved destination mismatch: got %q want %q", resolved, addr)
	}
}

func TestResolveTransferDestination_UsernameResolved(t *testing.T) {
	resolvedAddr := "dero1resolved"
	parse := func(s string) (*rpc.Address, error) {
		if s == resolvedAddr {
			return &rpc.Address{}, nil
		}
		return nil, fmt.Errorf("invalid")
	}
	resolved, _, err := resolveTransferDestination("alice", parse, func(name string) (string, error) {
		if name != "alice" {
			return "", fmt.Errorf("unexpected username %q", name)
		}
		return resolvedAddr, nil
	})
	if err != nil {
		t.Fatalf("expected username resolution to succeed, got error: %v", err)
	}
	if resolved != resolvedAddr {
		t.Fatalf("resolved destination mismatch: got %q want %q", resolved, resolvedAddr)
	}
}

func TestResolveTransferDestination_InvalidUsername(t *testing.T) {
	parse := func(s string) (*rpc.Address, error) {
		return nil, fmt.Errorf("invalid")
	}
	_, _, err := resolveTransferDestination("@alice", parse, func(name string) (string, error) {
		return "", nil
	})
	if err == nil {
		t.Fatal("expected invalid username to fail")
	}
	if !strings.Contains(err.Error(), "invalid DERO address or username") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestValidateKey tests hex key validation (used in RestoreFromKey)
func TestValidateKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "valid 64 char hex lowercase",
			key:     "0000000000000000000000000000000000000000000000000000000000000000",
			wantErr: false,
		},
		{
			name:    "valid 64 char hex mixed",
			key:     "ABCDEF1234567890abcdef1234567890ABCDEF1234567890abcdef1234567890",
			wantErr: false,
		},
		{
			name:    "valid 64 char hex all f",
			key:     "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			wantErr: false,
		},
		{
			name:    "too short 63 chars",
			key:     "000000000000000000000000000000000000000000000000000000000000000",
			wantErr: true,
		},
		{
			name:    "too long 65 chars",
			key:     "00000000000000000000000000000000000000000000000000000000000000000",
			wantErr: true,
		},
		{
			name:    "invalid hex char g",
			key:     "000000000000000000000000000000000000000000000000000000000000000g",
			wantErr: true,
		},
		{
			name:    "invalid hex char z",
			key:     "000000000000000000000000000000000000000000000000000000000000000z",
			wantErr: true,
		},
		{
			name:    "invalid hex char space",
			key:     "000000000000000000000000000000000000000000000000000000000000000 ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// validateKey validates a hex key string (test version without import cycle)
func validateKey(key string) error {
	key = strings.TrimSpace(key)
	if len(key) != 64 {
		return fmt.Errorf("key must be exactly 64 hexadecimal characters")
	}
	_, err := hex.DecodeString(key)
	if err != nil {
		return fmt.Errorf("invalid hex key: %w", err)
	}
	return nil
}

// containsString checks if s contains substring
func containsString(s, substring string) bool {
	return len(s) >= len(substring) && (s == substring || len(s) > 0 && containsSubstring(s, substring))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
