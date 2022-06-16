// Copyright 2022 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"math/big"

	"github.com/ethereum-optimism/optimism/op-bindings/predeploys"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

var big10 = big.NewInt(10)

var (
	L1BaseFeeSlot = common.BigToHash(big.NewInt(1))
	OverheadSlot  = common.BigToHash(big.NewInt(3))
	ScalarSlot    = common.BigToHash(big.NewInt(4))
	DecimalsSlot  = common.BigToHash(big.NewInt(5))
)

// calculateL1GasUsed returns the gas used to include the transaction data in
// the calldata on L1.
func calculateL1GasUsed(data []byte, overhead *big.Int) *big.Int {
	var zeroes uint64
	var ones uint64
	for _, byt := range data {
		if byt == 0 {
			zeroes++
		} else {
			ones++
		}
	}

	zeroesGas := zeroes * params.TxDataZeroGas
	onesGas := (ones + 68) * params.TxDataNonZeroGasEIP2028
	l1Gas := new(big.Int).SetUint64(zeroesGas + onesGas)
	return new(big.Int).Add(l1Gas, overhead)
}

// L1FeeContext includes all the context necessary to calculate the cost of
// including the transaction in L1.
type L1FeeContext struct {
	BaseFee  *big.Int
	Overhead *big.Int
	Scalar   *big.Int
	Decimals *big.Int
	Divisor  *big.Int
}

// NewL1FeeContext returns a context for calculating L1 fee cost.
// This depends on the oracles because gas costs can change over time.
func NewL1FeeContext(statedb *state.StateDB) *L1FeeContext {
	// TODO: unpack values after #2596
	// see: https://github.com/ethereum-optimism/optimism/pull/2596
	l1BaseFee := statedb.GetState(common.HexToAddress(predeploys.L1Block), L1BaseFeeSlot).Big()
	overhead := statedb.GetState(common.HexToAddress(predeploys.OVM_GasPriceOracle), OverheadSlot).Big()
	scalar := statedb.GetState(common.HexToAddress(predeploys.OVM_GasPriceOracle), ScalarSlot).Big()
	decimals := statedb.GetState(common.HexToAddress(predeploys.OVM_GasPriceOracle), DecimalsSlot).Big()
	divisor := new(big.Int).Exp(big10, decimals, nil)

	return &L1FeeContext{
		BaseFee:  l1BaseFee,
		Overhead: overhead,
		Scalar:   scalar,
		Decimals: decimals,
		Divisor:  divisor,
	}
}

// L1Cost returns the L1 fee cost.
func L1Cost(tx *types.Transaction, ctx *L1FeeContext) *big.Int {
	bytes, err := tx.MarshalBinary()
	if err != nil {
		panic(err)
	}
	l1GasUsed := calculateL1GasUsed(bytes, ctx.Overhead)
	l1Cost := new(big.Int).Mul(l1GasUsed, ctx.BaseFee)
	l1Cost = l1Cost.Mul(l1Cost, ctx.Scalar)
	l1Cost = l1Cost.Div(l1Cost, ctx.Divisor)
	return l1Cost
}
