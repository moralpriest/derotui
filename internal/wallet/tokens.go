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
	"github.com/deroproject/derohe/walletapi"
)

var (
	tokenFnRe          = regexp.MustCompile(`(?is)Function\s+(Name|Symbol|Ticker|Decimals)\s*\(\s*\)\s*(String|Uint64)(.*?)End Function`)
	tokenReturnLoadRe  = regexp.MustCompile(`(?i)RETURN\s+LOAD\s*\(\s*"([^"]+)"\s*\)`)
	tokenReturnQuoteRe = regexp.MustCompile(`(?i)RETURN\s+"([^"]+)"`)
	tokenReturnUintRe  = regexp.MustCompile(`(?i)RETURN\s+(\d+)`)
)

var tokenMetaKeys = []string{
	"name", "Name", "symbol", "Symbol", "ticker", "Ticker",
	"decimals", "Decimals", "n", "s", "d",
	"metadata", "var_header_name", "nameHdr",
}

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
	// EntriesNative is populated by TokenAdd before balances are decrypted;
	// include those SCIDs too so a token is visible while its first sync is
	// still in progress.
	tracked := make([]crypto.Hash, 0)
	for scid := range w.wallet.GetAccount().EntriesNative {
		tracked = append(tracked, scid)
	}
	for _, scid := range tracked {
		if _, ok := balances[scid]; !ok {
			balances[scid] = 0
		}
	}
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

// ProbeTokenBalance decrypts the wallet address's balance for a SCID
// straight from the daemon (single RPC) WITHOUT registering the token with
// the wallet. This is the token-discovery primitive: registration is only
// worthwhile once a balance is known to exist, because every registration
// burdens the wallet's sync loop forever (there is no TokenRemove upstream).
func (w *Wallet) ProbeTokenBalance(scidStr string) (uint64, error) {
	if w.wallet == nil {
		return 0, fmt.Errorf("wallet not open")
	}
	scid, err := parseSCID(scidStr)
	if err != nil {
		return 0, err
	}
	// Use the wallet's own sync height as topo point: it is always a valid
	// height and matches what the wallet considers "current".
	topo := int64(w.wallet.Get_Height())
	balance, _, err := w.wallet.GetDecryptedBalanceAtTopoHeight(scid, topo, w.wallet.GetAddress().String())
	return balance, err
}

// DiscoverTokenBalance probes a SCID's balance for this wallet. When the
// wallet holds the token it is registered (TokenAdd) so the wallet tracks
// it from then on; zero-balance candidates are left unregistered.
func (w *Wallet) DiscoverTokenBalance(scidStr string) (TokenInfo, bool) {
	scid := strings.ToLower(strings.TrimSpace(scidStr))
	balance, err := w.ProbeTokenBalance(scid)
	if err != nil {
		return TokenInfo{SCID: scid}, false
	}
	info := TokenInfo{SCID: scid, Balance: balance}
	if balance > 0 {
		_ = w.AddToken(scid)
		if n, t, d, ok := TokenMetadataFromStore(scid); ok {
			info.Name, info.Ticker, info.Decimals = n, t, d
		}
	}
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

// GetTokenMetadata fetches token name/ticker/decimals via DERO.GetSC.
// It parses code (Name/Symbol/Ticker/Decimals) and falls back to targeted
// key lookups (keysstring) for tokens that store metadata in state. Using
// keysstring avoids variables:true which dumps the entire SC tree and times
// out on popular tokens.
func (w *Wallet) GetTokenMetadata(ctx context.Context, scidStr string) (TokenInfo, error) {
	info := TokenInfo{SCID: strings.TrimSpace(scidStr)}
	if err := ValidateSCID(info.SCID); err != nil {
		return info, err
	}
	// Fast path: the local HyperGnomon store already indexed this SC's
	// name/ticker/decimals. Reading them is a local bbolt lookup instead of a
	// daemon RPC (DERO.GetSC with Code:true can be the slowest call in the
	// whole scan — up to the 8s timeout on large contracts). Only fall back
	// to the daemon when the store has nothing displayable.
	if n, t, d, ok := TokenMetadataFromStore(info.SCID); ok && (n != "" || t != "") {
		info.Name, info.Ticker, info.Decimals = n, t, d
		return info, nil
	}
	daemonAddr := w.GetDaemonAddress()
	if daemonAddr == "" || daemonAddr == "Not connected" {
		if walletapi.Daemon_Endpoint_Active != "" {
			daemonAddr = walletapi.Daemon_Endpoint_Active
		} else {
			if n, t, d, ok := TokenMetadataFromStore(info.SCID); ok {
				info.Name, info.Ticker, info.Decimals = n, t, d
				return info, nil
			}
			return info, nil
		}
	}
	rpcURL, err := daemonRPCURL(daemonAddr)
	if err != nil {
		if n, t, d, ok := TokenMetadataFromStore(info.SCID); ok {
			info.Name, info.Ticker, info.Decimals = n, t, d
		}
		return info, nil
	}
	// Targeted lookup: code + specific keys. Order matters for ValuesString.
	keys := tokenMetaKeys
	params := rpc.GetSC_Params{
		SCID:       info.SCID,
		Code:       true,
		KeysString: keys,
	}
	payloadObj := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "DERO.GetSC",
		"params":  params,
	}
	bodyBytes, err := json.Marshal(payloadObj)
	if err != nil {
		if n, t, d, ok := TokenMetadataFromStore(info.SCID); ok {
			info.Name, info.Ticker, info.Decimals = n, t, d
		}
		return info, nil
	}
	reqCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, "POST", rpcURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		if n, t, d, ok := TokenMetadataFromStore(info.SCID); ok {
			info.Name, info.Ticker, info.Decimals = n, t, d
		}
		return info, nil
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := sharedHTTPClient.Do(req)
	if err != nil {
		if n, t, d, ok := TokenMetadataFromStore(info.SCID); ok {
			info.Name, info.Ticker, info.Decimals = n, t, d
		}
		return info, nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if n, t, d, ok := TokenMetadataFromStore(info.SCID); ok {
			info.Name, info.Ticker, info.Decimals = n, t, d
		}
		return info, nil
	}
	var rpcResp struct {
		Result rpc.GetSC_Result `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		if n, t, d, ok := TokenMetadataFromStore(info.SCID); ok {
			info.Name, info.Ticker, info.Decimals = n, t, d
		}
		return info, nil
	}
	if rpcResp.Error != nil {
		if n, t, d, ok := TokenMetadataFromStore(info.SCID); ok {
			info.Name, info.Ticker, info.Decimals = n, t, d
		}
		return info, nil
	}
	vals := map[string]string{}
	for i, k := range keys {
		if i >= len(rpcResp.Result.ValuesString) {
			break
		}
		raw := rpcResp.Result.ValuesString[i]
		if raw == "" || strings.HasPrefix(raw, "NOT AVAILABLE") {
			continue
		}
		if s := decodeSCString(raw); s != "" {
			vals[strings.ToLower(k)] = s
		} else {
			vals[strings.ToLower(k)] = strings.TrimSpace(raw)
		}
	}
	applyTokenMetadata(&info, rpcResp.Result.Code, vals)
	if info.Name == "" && info.Ticker == "" {
		if n, t, d, ok := TokenMetadataFromStore(info.SCID); ok {
			if n != "" {
				info.Name = n
			}
			if t != "" {
				info.Ticker = t
			}
			if d != 0 && info.Decimals == 0 {
				info.Decimals = d
			}
		}
	}
	return info, nil
}

func applyTokenMetadata(info *TokenInfo, code string, vals map[string]string) {
	loadNameKey, loadTickerKey := "", ""
	for _, m := range tokenFnRe.FindAllStringSubmatch(code, -1) {
		if len(m) < 4 {
			continue
		}
		fn, body := strings.ToLower(m[1]), m[3]
		if lm := tokenReturnLoadRe.FindStringSubmatch(body); len(lm) == 2 {
			switch fn {
			case "name":
				loadNameKey = lm[1]
			case "symbol", "ticker":
				loadTickerKey = lm[1]
			}
			continue
		}
		if qm := tokenReturnQuoteRe.FindStringSubmatch(body); len(qm) == 2 {
			switch fn {
			case "name":
				if info.Name == "" {
					info.Name = qm[1]
				}
			case "symbol", "ticker":
				if info.Ticker == "" {
					info.Ticker = qm[1]
				}
			}
			continue
		}
		if fn == "decimals" {
			if um := tokenReturnUintRe.FindStringSubmatch(body); len(um) == 2 && info.Decimals == 0 {
				if d, err := strconv.ParseUint(um[1], 10, 64); err == nil {
					info.Decimals = d
				}
			}
		}
	}
	setFromVals := func(keys []string, dst *string) {
		if *dst != "" {
			return
		}
		for _, k := range keys {
			if v := strings.TrimSpace(vals[k]); v != "" && !strings.HasPrefix(v, "NOT AVAILABLE") {
				*dst = v
				return
			}
		}
	}
	if loadNameKey != "" && info.Name == "" {
		if v := strings.TrimSpace(vals[strings.ToLower(loadNameKey)]); v != "" {
			info.Name = v
		}
	}
	if loadTickerKey != "" && info.Ticker == "" {
		if v := strings.TrimSpace(vals[strings.ToLower(loadTickerKey)]); v != "" {
			info.Ticker = v
		}
	}
	setFromVals([]string{"name", "var_header_name", "namehdr", "n"}, &info.Name)
	setFromVals([]string{"symbol", "ticker", "s"}, &info.Ticker)
	if info.Decimals == 0 {
		for _, k := range []string{"decimals", "d"} {
			if v := strings.TrimSpace(vals[k]); v != "" {
				if d, err := strconv.ParseUint(v, 10, 64); err == nil {
					info.Decimals = d
					break
				}
			}
		}
	}
	if meta := vals["metadata"]; meta != "" && (info.Name == "" || info.Ticker == "") {
		var blob map[string]interface{}
		if err := json.Unmarshal([]byte(meta), &blob); err == nil {
			if info.Name == "" {
				if s, _ := blob["name"].(string); strings.TrimSpace(s) != "" {
					info.Name = strings.TrimSpace(s)
				}
			}
			if info.Ticker == "" {
				if s, _ := blob["symbol"].(string); strings.TrimSpace(s) != "" {
					info.Ticker = strings.TrimSpace(s)
				}
			}
		}
	}
}

func decodeSCString(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "NOT AVAILABLE") {
		return ""
	}
	if b, err := hex.DecodeString(s); err == nil {
		decoded := string(b)
		printable := true
		for _, r := range decoded {
			if r < 32 || r == 127 {
				printable = false
				break
			}
		}
		if printable && strings.TrimSpace(decoded) != "" {
			return strings.TrimSpace(decoded)
		}
	}
	return s
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
