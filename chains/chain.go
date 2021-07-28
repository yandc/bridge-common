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

package chains

import (
	"fmt"
	"sync"
	"time"

	"github.com/beego/beego/v2/core/logs"
)

type SDK interface {
	GetLatestHeight() (uint64, error)
	Address() string
}

type ChainSDK struct {
	sdk      SDK
	ChainID  uint64
	nodes    []SDK
	state    []bool
	index    int
	cursor   int
	status   int // SDK nodes status: 1. available, 0. all down
	height   uint64
	interval time.Duration
	maxGap   uint64
	sync.RWMutex
}

func (s *ChainSDK) Height() uint64 {
	s.RLock()
	defer s.RUnlock()
	return s.height
}

func (s *ChainSDK) WaitTillHeight(height uint64, interval time.Duration) {
	if interval == 0 {
		interval = s.interval
	}
	for {
		h, err := s.Node().GetLatestHeight()
		if err != nil {
			logs.Error("Failed to get chain(%v) latest height err %v", s.ChainID, err)
		} else if h >= height {
			return
		}
		time.Sleep(interval)
	}
}

func (s *ChainSDK) updateSelection() {
	var height uint64
	var sdk SDK
	var index int
	state := make([]uint64, len(s.nodes))
	for i, s := range s.nodes {
		h, err := s.GetLatestHeight()
		if err != nil {
			logs.Error("Node(%s) error %v", s.Address(), err)
		} else {
			state[i] = h
			if h > height {
				height = h
				sdk = s
				index = i
			}
		}
	}
	status := 1
	if sdk == nil {
		status = 0
		logs.Warn("Temp unavailabitlity for all node of chain %s", s.ChainID)
		if len(s.nodes) > 0 {
			sdk = s.nodes[0]
		}
	}
	s.Lock()
	s.sdk = sdk
	s.status = status
	s.height = height
	s.index = index
	for i, h := range state {
		s.state[i] = h >= height-s.maxGap
	}
	s.Unlock()
}

func (s *ChainSDK) Available() bool {
	s.RLock()
	defer s.RUnlock()
	return s.status > 0
}

func (s *ChainSDK) Index() int {
	s.RLock()
	defer s.RUnlock()
	return s.index
}

func (s *ChainSDK) Select() int {
	s.RLock()
	defer s.RUnlock()
	cursor := s.cursor % len(s.nodes)
	s.cursor++
	c := s.cursor % len(s.nodes)
	for c != cursor {
		if s.state[c] {
			break
		}
		s.cursor++
		c = s.cursor % len(s.nodes)
	}
	s.cursor = c
	return c
}

func (s *ChainSDK) Node() SDK {
	s.RLock()
	defer s.RUnlock()
	return s.sdk
}

func (s *ChainSDK) monitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		s.updateSelection()
	}
}

func (s *ChainSDK) Init() error {
	logs.Info("Initializing chain sdk for %v", s.ChainID)
	s.updateSelection()
	if !s.Available() {
		return fmt.Errorf("All the nodes are unavailable for chain %v", s.ChainID)
	}
	return nil
}

func New(chainID uint64, urls []string, interval time.Duration, maxGap uint64, f func(string) SDK) (sdk *ChainSDK, err error) {
	nodes := make([]SDK, len(urls))
	for i, url := range urls {
		nodes[i] = f(url)
	}
	return NewChainSDK(chainID, nodes, interval, maxGap)
}

func NewChainSDK(chainID uint64, nodes []SDK, interval time.Duration, maxGap uint64) (sdk *ChainSDK, err error) {
	var s SDK
	sdk = &ChainSDK{
		sdk:      s,
		ChainID:  chainID,
		nodes:    nodes,
		interval: interval,
		maxGap:   maxGap,
		state:    make([]bool, len(nodes)),
	}
	err = sdk.Init()
	if err == nil {
		go sdk.monitor(interval)
	}
	return
}