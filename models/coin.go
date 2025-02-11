package models

import "gorm.io/gorm"

type Coin struct {
	BaseModel
	Name       string `gorm:"index:idx_coin_name" json:"name"`
	Network    string `json:"network"`
	Unit       string `json:"unit"`
	UnitFactor uint64 `json:"unit_factor"`
}

func (coin *Coin) Migrate(db *gorm.DB) {
	db.AutoMigrate(&Coin{})
}
