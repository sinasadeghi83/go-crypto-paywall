package models

import "gorm.io/gorm"

type Transaction struct {
	BaseModel
	SrcAddr string `json:"src_addr"`
	DstAddr string `json:"dst_addr"`
	CoinID  int    `json:"coin_id"`
	Coin    Coin
	Amount  uint64 `json:"amount"`
	TxHash  string `json:"tx_hash"`
	Memo    string `json:"memo" gorm:"index"`
	LT      uint64 `json:"lt"`
}

func (tx *Transaction) Migrate(db *gorm.DB) {
	db.AutoMigrate(&Transaction{})
}
