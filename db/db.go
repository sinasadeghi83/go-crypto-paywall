package database

import (
	"github.com/sinasadeghi83/go-crypto-paywall/models"
	"gorm.io/gorm"
)

var (
	db *gorm.DB
)

func Setup(dbInstance *gorm.DB) {
	db = dbInstance
	MigrateAll(dbInstance)
}

func GetDB() *gorm.DB {
	return db
}

func MigrateAll(db *gorm.DB) {
	migrations := []models.Model{
		&models.Coin{},
		&models.Transaction{},
		&models.CryptoWallet{},
		&models.Invoice{},
	}

	for _, migration := range migrations {
		migration.Migrate(db)
	}
}
