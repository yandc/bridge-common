package wallet

import (
	"github.com/polynetwork/bridge-common/chains/benfen"
)

type BenfenWallet struct {
	sdk        *benfen.SDK
	Address    string
	PrivateKey string
	config     *Config
}

func NewBenfenWallet(config *Config, sdk *benfen.SDK) *BenfenWallet {
	return &BenfenWallet{sdk: sdk, Address: config.Address, PrivateKey: config.PrivateKey, config: config}
}

func (w *BenfenWallet) Init() (err error) {
	return
}
