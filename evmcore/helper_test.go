// Copyright 2015 The go-ethereum Authors
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

package evmcore

import (
	"container/list"

	"github.com/sesanetwork/go-sesa/core/rawdb"
	"github.com/sesanetwork/go-sesa/ethdb"
	notify "github.com/sesanetwork/go-sesa/event"
)

// Implement our EthTest Manager
type TestManager struct {
	// stateManager *StateManager
	eventMux *notify.TypeMux

	db     ethdb.Database
	txPool *TxPool
	bc     DummyChain
	Blocks []*EvmBlock
}

func (tm *TestManager) IsListening() bool {
	return false
}

func (tm *TestManager) IsMining() bool {
	return false
}

func (tm *TestManager) PeerCount() int {
	return 0
}

func (tm *TestManager) Peers() *list.List {
	return list.New()
}

func (tm *TestManager) BlockChain() DummyChain {
	return tm.bc
}

func (tm *TestManager) TxPool() *TxPool {
	return tm.txPool
}

// func (tm *TestManager) StateManager() *StateManager {
// 	return tm.stateManager
// }

func (tm *TestManager) EventMux() *notify.TypeMux {
	return tm.eventMux
}

// func (tm *TestManager) KeyManager() *crypto.KeyManager {
// 	return nil
// }

func (tm *TestManager) Db() ethdb.Database {
	return tm.db
}

func NewTestManager() *TestManager {
	testManager := &TestManager{}
	testManager.eventMux = new(notify.TypeMux)
	testManager.db = rawdb.NewMemoryDatabase()
	// testManager.txPool = NewTxPool(testManager)
	// testManager.blockChain = NewBlockChain(testManager)
	// testManager.stateManager = NewStateManager(testManager)
	return testManager
}
