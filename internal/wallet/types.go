// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

// WalletInfo contains wallet information
type WalletInfo struct {
	Address           string
	Balance           uint64
	LockedBalance     uint64
	Height            uint64
	DaemonHeight      uint64
	TopoHeight        int64
	IsOnline          bool
	IsSynced          bool
	IsRegistered      bool
	Network           string
	DaemonAddress     string
	IntegratedAddress string
}

// TransactionInfo contains transaction details
type TransactionInfo struct {
	TxID            string
	Amount          int64 // positive = incoming, negative = outgoing
	Fee             uint64
	Height          uint64
	TopoHeight      int64
	Timestamp       int64
	PaymentID       string
	Destination     string
	Coinbase        bool // true if miner reward
	Incoming        bool // true if incoming transaction
	BlockHash       string
	Proof           string // payment proof
	Sender          string
	Burn            uint64
	DestinationPort uint64 // for SC calls
	SourcePort      uint64 // for SC calls
	Status          byte   // 0=confirmed, 1=spent, 2=unknown
	Message         string // transaction message/comment from Payload_RPC
}

// TransferParams contains transfer parameters
type TransferParams struct {
	Destination string
	Amount      uint64
	PaymentID   uint64
	Ringsize    uint64
	Message     string
}

// TransferResult contains transfer result
type TransferResult struct {
	TxID   string
	Fee    uint64
	Status string
	Error  string
}
