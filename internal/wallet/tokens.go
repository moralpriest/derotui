// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/rpc"
)

// tokenCodeNameRe matches Function Name() String ... RETURN "..."
var tokenCodeNameRe = regexp.MustCompile(`(?i)Function\s+Name\s*\(\s*\)\s*String[\s\S]*?RETURN[^\n"]*"([^"]+)"`)

// tokenCodeTickerRe matches Function Ticker() String ... RETURN "..."
var tokenCodeTickerRe = regexp.MustCompile(`(?i)Function\s+Ticker\s*\(\s*\)\s*String[\s\S]*?RETURN[^\n"]*"([^"]+)"`)

// tokenCodeDecimalsRe matches Function Decimals() Uint64 ... RETURN <number>
var tokenCodeDecimalsRe = regexp.MustCompile(`(?i)Function\s+Decimals\s*\(\s*\)\s*Uint64[\s\S]*?RETURN[^\d]*(\d+)`)

// ValidateSCID checks whether s is a 64-char hex SCID.
func ValidateSCID(s string) error {
	s = strings.TrimSpace(s)
	if len(s) != 64 {
		return fmt.Errorf("SCID must be 64 hex characters (got %d)", len(s))
	}
	if _, err := hex.DecodeString(s); err != nil {
		return fmt.Errorf("SCID is not valid hex: %v", err)
	}
	return nil
}

// parseSCID converts a hex SCID string to crypto.Hash.
func parseSCID(s string) (crypto.Hash, error) {
	s = strings.TrimSpace(s)
	if err := ValidateSCID(s); err != nil {
		return crypto.Hash{}, err
	}
	b, _ := hex.DecodeString(s)
	var h crypto.Hash
	copy(h[:], b)
	return h, nil
}

// FormatTokenAmount formats a token balance with the given decimals.
func FormatTokenAmount(balance uint64, decimals uint64) string {
	if decimals == 0 {
		return strconv.FormatUint(balance, 10)
	}
	divisor := uint64(1)
	for i := uint64(0); i < decimals; i++ {
		divisor *= 10
	}
	whole := balance / divisor
	frac := balance % divisor
	fracStr := fmt.Sprintf("%0*d", decimals, frac)
	fracStr = strings.TrimRight(fracStr, "0")
	if fracStr == "" {
		return fmt.Sprintf("%d", whole)
	}
	return fmt.Sprintf("%d.%s", whole, fracStr)
}

// ParseTokenAmount parses a decimal string into atomic units for the given decimals.
func ParseTokenAmount(s string, decimals uint64) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("amount is empty")
	}
	if strings.HasPrefix(s, "-") {
		return 0, fmt.Errorf("amount cannot be negative")
	}
	parts := strings.SplitN(s, ".", 2)
	wholeStr := parts[0]
	if wholeStr == "" {
		wholeStr = "0"
	}
	whole, err := strconv.ParseUint(wholeStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount: %v", err)
	}
	divisor := uint64(1)
	for i := uint64(0); i < decimals; i++ {
		divisor *= 10
	}
	if len(parts) == 1 {
		return whole * divisor, nil
	}
	fracStr := parts[1]
	if uint64(len(fracStr)) > decimals {
		return 0, fmt.Errorf("too many decimal places (max %d)", decimals)
	}
	for uint64(len(fracStr)) < decimals {
		fracStr += "0"
	}
	frac, err := strconv.ParseUint(fracStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid fractional amount: %v", err)
	}
	if whole > (^uint64(0)-frac)/divisor {
		return 0, fmt.Errorf("amount overflows")
	}
	return whole*divisor + frac, nil
}

// ListTokens returns tokens known to the wallet (from decrypted balances).
// Zero-hash (base DERO) is excluded. TokenAdd must be called before the
// wallet API exposes a token balance, so the UI also keeps explicitly tracked
// SCIDs as a fallback for zero-balance tokens.
func (w *Wallet) ListTokens() []TokenInfo {
	if w.wallet == nil {
		return nil
	}
	balances := w.wallet.Balances()
	var out []TokenInfo
	var zero crypto.Hash
	for scid, bal := range balances {
		if scid == zero {
			continue
		}
		out = append(out, TokenInfo{
			SCID:    scid.String(),
			Balance: bal,
		})
	}
	return out
}

// DiscoverTokenBalance registers a SCID and synchronously refreshes its
// encrypted balance. It is used by the UI's background token scan.
func (w *Wallet) DiscoverTokenBalance(scidStr string) (TokenInfo, bool) {
	if err := w.AddToken(scidStr); err != nil {
		return TokenInfo{}, false
	}
	info := TokenInfo{SCID: strings.ToLower(strings.TrimSpace(scidStr))}
	balance, err := w.GetTokenBalance(info.SCID)
	if err != nil {
		return info, false
	}
	info.Balance = balance
	return info, true
}

// AddToken registers a token SCID for tracking.
// The wallet will start
// syncing its history on the next daemon sync round.
func (w *Wallet) AddToken(scidStr string) error {
	if w.wallet == nil {
		return fmt.Errorf("wallet not open")
	}
	scid, err := parseSCID(scidStr)
	if err != nil {
		return err
	}
	if err := w.wallet.TokenAdd(scid); err != nil {
		return fmt.Errorf("failed to add token: %v", err)
	}
	return nil
}

// GetTokenMetadata fetches token name/ticker/decimals via DERO.GetSC code parsing.
// Returns a TokenInfo with SCID populated and metadata filled where available.
// If daemon is unreachable or parsing fails, it returns the SCID with defaults
// (decimals=0) and no error, so callers can still display the token.
func (w *Wallet) GetTokenMetadata(ctx context.Context, scidStr string) (TokenInfo, error) {
	info := TokenInfo{SCID: strings.TrimSpace(scidStr)}
	if err := ValidateSCID(info.SCID); err != nil {
		return info, err
	}
	daemonAddr := w.GetDaemonAddress()
	if daemonAddr == "" || daemonAddr == "Not connected" {
		return info, nil
	}
	url, err := daemonRPCURL(daemonAddr)
	if err != nil {
		return info, nil
	}
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","id":"1","method":"DERO.GetSC","params":{"scid":"%s","code":true,"variables":false}}`, info.SCID)
	reqCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, "POST", url, strings.NewReader(payload))
	if err != nil {
		return info, nil
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return info, nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return info, nil
	}
	var rpcResp struct {
		Result struct {
			Code string `json:"code"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return info, nil
	}
	if rpcResp.Error != nil {
		return info, nil
	}
	code := rpcResp.Result.Code
	if code == "" {
		return info, nil
	}
	if m := tokenCodeNameRe.FindStringSubmatch(code); len(m) == 2 {
		info.Name = m[1]
	}
	if m := tokenCodeTickerRe.FindStringSubmatch(code); len(m) == 2 {
		info.Ticker = m[1]
	}
	if m := tokenCodeDecimalsRe.FindStringSubmatch(code); len(m) == 2 {
		if d, err := strconv.ParseUint(m[1], 10, 64); err == nil {
			info.Decimals = d
		}
	}
	return info, nil
}

// TransferToken sends a token to a destination.
func (w *Wallet) TransferToken(params TokenTransferParams) TransferResult {
	if w.wallet == nil {
		return TransferResult{Error: "wallet not open"}
	}
	scid, err := parseSCID(params.SCID)
	if err != nil {
		return TransferResult{Error: err.Error()}
	}
	deroBalance, _ := w.wallet.Get_Balance()
	const minFee uint64 = 2000
	if deroBalance < minFee {
		return TransferResult{Error: fmt.Sprintf("insufficient DERO for fee: have %.5f DERO, need at least %.5f", float64(deroBalance)/100000, float64(minFee)/100000)}
	}
	tokenBal, _ := w.wallet.Get_Balance_scid(scid)
	if params.Amount > tokenBal {
		return TransferResult{Error: fmt.Sprintf("insufficient token balance: have %d, need %d", tokenBal, params.Amount)}
	}
	resolvedDestination, addr, err := resolveTransferDestination(
		params.Destination,
		globals.ParseValidateAddress,
		func(name string) (string, error) {
			return w.wallet.NameToAddress(name)
		},
	)
	if err != nil {
		return TransferResult{Error: err.Error()}
	}

	ringsize := params.Ringsize
	if ringsize == 0 {
		if w.simulator {
			ringsize = 2
		} else {
			ringsize = 16
		}
	} else if ringsize > 128 {
		ringsize = 128
	} else if !isPowerOf2(int(ringsize)) {
		if w.simulator {
			ringsize = 2
		} else {
			ringsize = 16
		}
	}

	var arguments rpc.Arguments
	if addr != nil && addr.IsIntegratedAddress() {
		for _, arg := range addr.Arguments {
			arguments = append(arguments, arg)
		}
		if err := arguments.Validate_Arguments(); err != nil {
			return TransferResult{Error: fmt.Sprintf("integrated address has invalid arguments: %v", err)}
		}
	}
	if len(arguments) > 0 {
		if _, err := arguments.CheckPack(144); err != nil {
			return TransferResult{Error: fmt.Sprintf("payload arguments too large (max 144 bytes): %v", err)}
		}
	}

	transfers := []rpc.Transfer{
		{
			SCID:        scid,
			Destination: resolvedDestination,
			Amount:      params.Amount,
			Payload_RPC: arguments,
		},
	}

	tx, err := w.wallet.TransferPayload0(transfers, ringsize, false, rpc.Arguments{}, 0, false)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "verification failed") {
			return TransferResult{Error: fmt.Sprintf("TX failed: %v (token_balance=%d, amount=%d, dero_balance=%d)", err, tokenBal, params.Amount, deroBalance)}
		}
		return TransferResult{Error: fmt.Sprintf("transfer failed: %v", err)}
	}
	txID := tx.GetHash().String()
	if err = w.wallet.SendTransaction(tx); err != nil {
		return TransferResult{Error: fmt.Sprintf("failed to dispatch transaction: %v", err)}
	}
	w.InvalidateTxCache()
	return TransferResult{TxID: txID, Status: "success"}
}

// GetTokenBalance returns the balance for a specific token SCID.
func (w *Wallet) GetTokenBalance(scidStr string) (uint64, error) {
	if w.wallet == nil {
		return 0, fmt.Errorf("wallet not open")
	}
	scid, err := parseSCID(scidStr)
	if err != nil {
		return 0, err
	}
	bal, _ := w.wallet.Get_Balance_scid(scid)
	return bal, nil
}

// GetTokenTransfers returns transactions for a specific token SCID.
func (w *Wallet) GetTokenTransfers(scidStr string, count int) []TransactionInfo {
	if w.wallet == nil {
		return nil
	}
	scid, err := parseSCID(scidStr)
	if err != nil {
		return nil
	}
	entries := w.wallet.Show_Transfers(scid, true, true, true, 0, 0, "", "", 0, 0)
	return w.getTransactionsForSCID(entries, count)
}

// getTransactionsForSCID maps rpc.Entry slice to TransactionInfo.
func (w *Wallet) getTransactionsForSCID(entries []rpc.Entry, count int) []TransactionInfo {
	n := len(entries)
	if count > 0 && n > count {
		entries = entries[n-count:]
		n = len(entries)
	}
	for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	out := make([]TransactionInfo, 0, n)
	for _, e := range entries {
		amt := int64(e.Amount)
		msg := ""
		for _, arg := range e.Payload_RPC {
			if arg.Name == rpc.RPC_COMMENT && arg.DataType == rpc.DataString {
				if s, ok := arg.Value.(string); ok {
					msg = s
				}
			}
		}
		ti := TransactionInfo{
			TxID:      e.TXID,
			Amount:    amt,
			Height:    e.Height,
			Timestamp: e.Time.Unix(),
			Message:   msg,
		}
		if w.wallet != nil && e.Destination != "" {
			if e.Destination == w.wallet.GetAddress().String() {
				ti.Incoming = true
			} else {
				ti.Incoming = false
				if ti.Amount > 0 {
					ti.Amount = -ti.Amount
				}
			}
		}
		out = append(out, ti)
	}
	return out
}
