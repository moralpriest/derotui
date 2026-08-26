// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/derohe/walletapi"
)

// NameServiceSCID is the hardcoded SCID of the DERO Name Service.
// It is a zero hash with byte 31 set to 1.
var NameServiceSCID crypto.Hash

func init() {
	NameServiceSCID[31] = 1
}

// NameEntry represents a registered name and its owner address.
type NameEntry struct {
	Name  string
	Owner string // Owner address as DERO address string
}

// validateNameForRegistration checks whether a candidate name is valid for the
// DERO Name Service.  Exported so tests can exercise name validation directly.
func validateNameForRegistration(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if len(name) >= 64 {
		return fmt.Errorf("name must be less than 64 characters")
	}
	return nil
}

// RegisterName registers a name for the wallet's address via the NameService SC.
// Returns the transaction ID on success.
func (w *Wallet) RegisterName(ctx context.Context, name string) (string, error) {
	if w.wallet == nil {
		return "", fmt.Errorf("wallet not open")
	}

	if err := validateNameForRegistration(name); err != nil {
		return "", err
	}

	if !walletapi.IsDaemonOnline() {
		return "", fmt.Errorf("daemon is offline")
	}

	if !w.wallet.GetMode() {
		w.wallet.SetOnlineMode()
	}

	// Build SC data for NameService.Register(name).
	scdata := rpc.Arguments{
		{Name: rpc.SCACTION, DataType: rpc.DataUint64, Value: uint64(rpc.SC_CALL)},
		{Name: rpc.SCID, DataType: rpc.DataHash, Value: NameServiceSCID},
		{Name: "entrypoint", DataType: rpc.DataString, Value: "Register"},
		{Name: "name", DataType: rpc.DataString, Value: strings.TrimSpace(name)},
	}

	transfers := w.scCallTransfers()

	tx, err := w.wallet.TransferPayload0(transfers, 0, false, scdata, 0, false)
	if err != nil {
		return "", fmt.Errorf("failed to build name registration transaction: %w", err)
	}

	txID := tx.GetHash().String()
	if err := w.wallet.SendTransaction(tx); err != nil {
		return txID, fmt.Errorf("failed to dispatch name registration: %w", err)
	}

	w.InvalidateTxCache()

	return txID, nil
}

// TransferName transfers ownership of a registered name to a new address.
// Returns the transaction ID on success.
func (w *Wallet) TransferName(ctx context.Context, name, newOwner string) (string, error) {
	if w.wallet == nil {
		return "", fmt.Errorf("wallet not open")
	}

	name = strings.TrimSpace(name)
	newOwner = strings.TrimSpace(newOwner)
	if name == "" {
		return "", fmt.Errorf("name cannot be empty")
	}
	if newOwner == "" {
		return "", fmt.Errorf("new owner address cannot be empty")
	}

	if !walletapi.IsDaemonOnline() {
		return "", fmt.Errorf("daemon is offline")
	}

	if !w.wallet.GetMode() {
		w.wallet.SetOnlineMode()
	}

	scdata := rpc.Arguments{
		{Name: rpc.SCACTION, DataType: rpc.DataUint64, Value: uint64(rpc.SC_CALL)},
		{Name: rpc.SCID, DataType: rpc.DataHash, Value: NameServiceSCID},
		{Name: "entrypoint", DataType: rpc.DataString, Value: "TransferOwnership"},
		{Name: "name", DataType: rpc.DataString, Value: name},
		{Name: "newowner", DataType: rpc.DataString, Value: newOwner},
	}

	transfers := w.scCallTransfers()

	tx, err := w.wallet.TransferPayload0(transfers, 0, false, scdata, 0, false)
	if err != nil {
		return "", fmt.Errorf("failed to build name transfer transaction: %w", err)
	}

	txID := tx.GetHash().String()
	if err := w.wallet.SendTransaction(tx); err != nil {
		return txID, fmt.Errorf("failed to dispatch name transfer: %w", err)
	}

	w.InvalidateTxCache()

	return txID, nil
}

// scCallTransfers builds a 0-value transfer list to a random ring member,
// which is required as the "dummy" destination for SC-only calls.
func (w *Wallet) scCallTransfers() []rpc.Transfer {
	var zeroscid crypto.Hash
	transfers := make([]rpc.Transfer, 0, 1)
	ringMembers := w.wallet.Random_ring_members(zeroscid)
	for _, k := range ringMembers {
		if k != w.wallet.GetAddress().String() {
			transfers = append(transfers, rpc.Transfer{Destination: k, Amount: 0})
			break
		}
	}
	if len(transfers) == 0 {
		transfers = append(transfers, rpc.Transfer{Destination: w.wallet.GetAddress().String(), Amount: 0})
	}
	return transfers
}

// filterOwnedNames extracts names owned by myAddr from a GetSC VariableStringKeys
// map.  Values are hex-encoded compressed public keys (33 bytes).  Reserved SC
// metadata keys and malformed/foreign entries are silently skipped.
func filterOwnedNames(varKeys map[string]interface{}, myAddr string, testnet bool) []NameEntry {
	var entries []NameEntry
	for name, valueRaw := range varKeys {
		if name == "owner" || name == "own1" || name == "C" {
			continue
		}

		valueStr, ok := valueRaw.(string)
		if !ok {
			continue
		}

		ownerBytes, err := hex.DecodeString(valueStr)
		if err != nil || len(ownerBytes) != 33 {
			continue
		}

		addr, err := rpc.NewAddressFromCompressedKeys(ownerBytes)
		if err != nil {
			continue
		}
		addr.Mainnet = !testnet

		if addr.String() != myAddr {
			continue
		}

		entries = append(entries, NameEntry{
			Name:  name,
			Owner: addr.String(),
		})
	}
	return entries
}

// ListRegisteredNames queries the NameService SC via the daemon's GetSC RPC
// and returns all names owned by the wallet's address.
func (w *Wallet) ListRegisteredNames(ctx context.Context, daemonAddr string) ([]NameEntry, error) {
	if w.wallet == nil {
		return nil, fmt.Errorf("wallet not open")
	}

	myAddr := w.wallet.GetAddress().String()

	rpcURL, err := daemonRPCURL(daemonAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid daemon address: %w", err)
	}

	reqBody := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":"1","method":"DERO.GetSC","params":{"scid":"%s","variables":true}}`,
		hex.EncodeToString(NameServiceSCID[:]),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, strings.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create GetSC request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := sharedHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query name service: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp struct {
		Error  *json.RawMessage `json:"error"`
		Result struct {
			VariableStringKeys map[string]interface{} `json:"stringkeys"`
			Status             string                 `json:"status"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("failed to decode GetSC response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("daemon returned error for GetSC")
	}

	if rpcResp.Result.Status != "OK" {
		return nil, fmt.Errorf("GetSC returned status: %s", rpcResp.Result.Status)
	}

	return filterOwnedNames(rpcResp.Result.VariableStringKeys, myAddr, w.testnet), nil
}
