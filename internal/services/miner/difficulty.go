// Copyright 2017-2026 DERO Project. All rights reserved.

package miner

import (
	"github.com/deroproject/derohe/cryptography/crypto"
	"math/big"
)

var (
	bigZero   = big.NewInt(0)
	bigOne    = big.NewInt(1)
	oneLsh256 = new(big.Int).Lsh(bigOne, 256)
)

func HashToBig(buf crypto.Hash) *big.Int {
	blen := len(buf)
	for i := 0; i < blen/2; i++ {
		buf[i], buf[blen-1-i] = buf[blen-1-i], buf[i]
	}
	return new(big.Int).SetBytes(buf[:])
}

func ConvertDifficultyToBig(difficultyi uint64) *big.Int {
	difficulty := new(big.Int).SetUint64(difficultyi)
	denominator := new(big.Int).Add(difficulty, bigZero)
	return new(big.Int).Div(oneLsh256, denominator)
}

func ConvertIntegerDifficultyToBig(difficultyi *big.Int) *big.Int {
	if difficultyi.Cmp(bigZero) == 0 {
		panic("difficulty can never be zero")
	}
	return new(big.Int).Div(oneLsh256, difficultyi)
}

func CheckPowHashBig(powHash crypto.Hash, bigDiff *big.Int) bool {
	bigPowHash := HashToBig(powHash)
	bigDiffTarget := ConvertIntegerDifficultyToBig(bigDiff)
	return bigPowHash.Cmp(bigDiffTarget) <= 0
}
