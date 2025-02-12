package models

import (
	"encoding/base64"
	"fmt"
	"net/url"

	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/tvm/cell"
	"gorm.io/gorm"
)

type CryptoStatus string

const (
	UndefinedCrypto CryptoStatus = "undefined"
	ActiveCrypto    CryptoStatus = "active"
	ReserveCrypto   CryptoStatus = "reserve"
)

type CryptoWallet struct {
	BaseModel
	Addr    string `json:"addr"`
	Network string `json:"network"`
	Status  string
}

func (crypto *CryptoWallet) TonURL(amount string, memo string) string {
	addr := address.MustParseAddr(crypto.Addr)
	body := cell.BeginCell().MustStoreUInt(0, 32).MustStoreStringSnake(memo).EndCell()

	return fmt.Sprintf("ton://transfer/%s?bin=%s&amount=%s", addr.String(),
		base64.URLEncoding.EncodeToString(body.ToBOC()), tlb.MustFromTON(amount).Nano().String())
}

func (crypto *CryptoWallet) TokenTonURL(amount, memo, jettonAddr string) string {
	// Construct the transfer URL
	transferURL := fmt.Sprintf("ton://transfer/%s?amount=%s&jetton=%s", url.PathEscape(crypto.Addr), amount, url.PathEscape(jettonAddr))

	// Append comment if provided
	if memo != "" {
		transferURL += "&text=" + url.QueryEscape(memo)
	}

	return transferURL
}

func (crypto *CryptoWallet) Migrate(db *gorm.DB) {
	db.AutoMigrate(&CryptoWallet{})
}
