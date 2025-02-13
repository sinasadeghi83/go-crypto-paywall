package models

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InvoiceStatus string

const (
	PendingInvoice   InvoiceStatus = "pending"
	HalfpaidInvoice  InvoiceStatus = "half-paid"
	ExpiredInvoice   InvoiceStatus = "expired"
	CancelledInvoice InvoiceStatus = "cancelled"
	PaidInvoice      InvoiceStatus = "paid"
)

type Invoice struct {
	BaseModel
	Memo         string        `gorm:"type:varchar(100);uniqueIndex" json:"memo"`
	Status       InvoiceStatus `json:"status"`
	Price        uint64        `json:"price"`
	CoinID       uint          `json:"coin_id"`
	Coin         Coin          `json:"-"`
	AcceptOthers bool          `gorm:"default:0" json:"accept_others"` //does this invoice accept other coins too?
	ExpiresAt    time.Time     `json:"expires_at"`
}

func (in *Invoice) Migrate(db *gorm.DB) {
	db.AutoMigrate(&Invoice{})
}

func GetURLByInvoiceID(db *gorm.DB, invoiceID uint) string {
	var invoice *Invoice
	db.Preload(clause.Associations).Find(&invoice, invoiceID)
	return invoice.GetURL(db)
}

func (invoice *Invoice) GetURL(db *gorm.DB) string {
	var w CryptoWallet
	db.Where("network = ?", invoice.Coin.Network).First(&w, "status = 'active'")
	amount := fmt.Sprintf("%d", invoice.Price)
	if invoice.Coin.Unit == "USDT" && invoice.Coin.Network == "TON" {
		usdt_addr := "EQCxE6mUtQJKFnGfaROTKOt1lZbDiiX1kCixRv7Nw2Id_sDs"
		if os.Getenv("env") != "prod" {
			usdt_addr = "kQD0GKBM8ZbryVk2aESmzfU6b9b_8era_IkvBSELujFZPsyy"
		}
		return w.TokenTonURL(amount, invoice.Memo, usdt_addr)
	}
	return w.TonURL(amount, invoice.Memo)
}

func (in *Invoice) Create(db *gorm.DB) error {
	for {
		in.Memo = generateRandomString(10)
		res := db.Find(&Invoice{}, "memo = ?", in.Memo)
		if res.RowsAffected == 0 {
			break
		}
	}

	in.Status = PendingInvoice
	res := db.Save(in)
	return res.Error
}

func (in *Invoice) CancelInvoice(db *gorm.DB) error {
	in.Status = CancelledInvoice
	if res := db.Save(in); res.Error != nil {
		return res.Error
	}

	return nil
}

func (in *Invoice) CheckExpire(db *gorm.DB) error {
	if time.Now().After(in.ExpiresAt) {
		in.Status = ExpiredInvoice
	}

	if res := db.Save(in); res.Error != nil {
		return res.Error
	}

	return nil
}

func (in *Invoice) ApplyTx(db *gorm.DB, tx Transaction) (error, error) {
	if err := in.CheckExpire(db); err != nil {
		return err, nil
	}
	if in.Status == ExpiredInvoice || in.Status == CancelledInvoice {
		return nil, fmt.Errorf("invoice is expired or cancelled")
	}

	if !in.AcceptOthers && in.CoinID != tx.CoinID {
		return nil, fmt.Errorf("coins doesn't match")
	}

	if in.AcceptOthers && in.CoinID != tx.CoinID {
		//TODO
		return nil, fmt.Errorf("not supported")
	}

	var txs []Transaction
	db.Find(&txs, "memo = ?", in.Memo)

	var cumulative uint64 = 0
	for _, t := range txs {
		cumulative += t.Amount
	}

	if cumulative >= in.Price {
		in.Status = PaidInvoice
	} else {
		in.Status = HalfpaidInvoice
	}

	if res := db.Save(in); res.Error != nil {
		return res.Error, nil
	}

	return nil, nil
}

// Generates a random string of specified length using math/rand (not cryptographically secure).
func generateRandomString(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}
