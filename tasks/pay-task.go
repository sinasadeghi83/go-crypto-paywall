package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hibiken/asynq"
	database "github.com/sinasadeghi83/go-crypto-paywall/db"
	"github.com/sinasadeghi83/go-crypto-paywall/models"
)

type HandlePaid func(models.Transaction, models.Invoice, models.CryptoWallet)

type TxDeliveryPayload struct {
	Tx     models.Transaction
	Wallet models.CryptoWallet
}

const (
	TypeTxDelivery = "transaction:delivery"
)

func NewTxDeliveryTask(tx models.Transaction, wallet models.CryptoWallet) (*asynq.Task, error) {
	payload, err := json.Marshal(TxDeliveryPayload{tx, wallet})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeTxDelivery, payload, asynq.MaxRetry(5), asynq.Timeout(20*time.Minute)), nil
}

func HandleTxDeliveryTask(userHandler HandlePaid) (handler func(context.Context, *asynq.Task) error) {
	return func(ctx context.Context, t *asynq.Task) error {
		db := database.GetDB()
		var p TxDeliveryPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
		}
		log.Println("Handling Transaction with id:", p.Tx.ID)

		var invoice *models.Invoice
		if res := db.First(&invoice, "memo = ?", p.Tx.Memo); res.Error != nil {
			return nil
		}

		err, msgErr := invoice.ApplyTx(db, p.Tx)

		if msgErr != nil {
			log.Println("Invoice & Transaction doesn't match err:", msgErr)
		} else {
			if invoice.Status == models.PaidInvoice {
				userHandler(p.Tx, *invoice, p.Wallet)
			}
		}
		return err
	}
}
