/*
 * Copyright (C) 2021 The poly network Authors
 * This file is part of The poly network library.
 *
 * The  poly network  is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The  poly network  is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 * You should have received a copy of the GNU Lesser General Public License
 * along with The poly network .  If not, see <http://www.gnu.org/licenses/>.
 */

package wallet

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/polynetwork/bridge-common/chains/eth"
)

type IWallet interface {
	Send(addr common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, gasPriceX *big.Float, data []byte) (err error)
}

type Wallet struct {
	sync.RWMutex
	chainId   uint64
	providers map[accounts.Account]Provider
	provider  Provider                           // active account provider
	account   accounts.Account                   // active account
	nonces    map[accounts.Account]NonceProvider // account nonces
	sdk       *eth.SDK
}

type Provider interface {
	SignTx(account accounts.Account, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error)
	Accounts() []accounts.Account
}

func (w *Wallet) Send(addr common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, gasPriceX *big.Float, data []byte) (err error) {
	if gasPrice == nil || gasPrice.Sign() <= 0 {
		gasPrice, err = w.GasPrice()
		if err != nil {
			return fmt.Errorf("Get gas price error %v", err)
		}
		if gasPriceX != nil {
			gasPrice, _ = new(big.Float).Mul(new(big.Float).SetInt(gasPrice), gasPriceX).Int(nil)
		}
	}

	account, provider, nonces := w.Account()
	nonce, err := nonces.Acquire()
	if err != nil {
		return err
	}
	if gasLimit == 0 {
		msg := ethereum.CallMsg{From: account.Address, To: &addr, GasPrice: gasPrice, Value: big.NewInt(0), Data: data}
		gasLimit, err = w.sdk.Node().EstimateGas(context.Background(), msg)
		if err != nil {
			nonces.Update(false)
			return fmt.Errorf("Estimate gas limit error %v", err)
		}
	}

	limit := GetChainGasLimit(w.chainId, gasLimit)
	if limit < gasLimit {
		nonces.Update(false)
		return fmt.Errorf("Send tx estimated gas limit(%v) higher than max %v", gasLimit, limit)
	}
	tx := types.NewTransaction(nonce, addr, amount, limit, gasPrice, data)
	tx, err = provider.SignTx(account, tx, big.NewInt(int64(w.chainId)))
	if err != nil {
		nonces.Update(false)
		return fmt.Errorf("Sign tx error %v", err)
	}
	err = w.sdk.Node().SendTransaction(context.Background(), tx)
	// Check err here before update nonces
	nonces.Update(true)
	return err
}

func (w *Wallet) Account() (accounts.Account, Provider, NonceProvider) {
	w.RLock()
	defer w.RUnlock()
	return w.account, w.provider, w.nonces[w.account]
}

func (w *Wallet) GasPrice() (price *big.Int, err error) {
	return
}
