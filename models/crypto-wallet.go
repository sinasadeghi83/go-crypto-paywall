package models

import "gorm.io/gorm"

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

func (crypto *CryptoWallet) Migrate(db *gorm.DB) {
	db.AutoMigrate(&CryptoWallet{})
}
