package listeners

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/hibiken/asynq"
	database "github.com/sinasadeghi83/go-crypto-paywall/db"
	"github.com/sinasadeghi83/go-crypto-paywall/models"
	"github.com/sinasadeghi83/go-crypto-paywall/queue"
	"github.com/sinasadeghi83/go-crypto-paywall/tasks"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/ton/jetton"
)

var (
	client *liteclient.ConnectionPool
)

func ListenOnTON(ctx context.Context) error {
	api, err := setupTONClient(ctx)
	if err != nil {
		return err
	}

	var wallets []models.CryptoWallet
	database.GetDB().Where("network = 'TON' and status = ?", models.ActiveCrypto).Find(&wallets)

	for _, wallet := range wallets {
		go listenTonWallet(ctx, api, wallet)
	}

	<-ctx.Done()
	return nil
}

func setupTONClient(ctx context.Context) (ton.APIClientWrapped, error) {
	client = liteclient.NewConnectionPool()
	netaddr := "https://ton.org/global.config.json"
	if os.Getenv("env") != "prod" {
		netaddr = "https://ton-blockchain.github.io/testnet-global.config.json"
	}

	cfg, err := liteclient.GetConfigFromUrl(ctx, netaddr)
	if err != nil {
		log.Fatalln("get config err: ", err.Error())
		return nil, err
	}

	// connect to mainnet lite servers
	err = client.AddConnectionsFromConfig(context.Background(), cfg)
	if err != nil {
		log.Fatalln("connection err: ", err.Error())
		return nil, err
	}

	// initialize ton api lite connection wrapper with full proof checks
	api := ton.NewAPIClient(client, ton.ProofCheckPolicyFast).WithRetry()
	api.SetTrustedBlockFromConfig(cfg)

	return api, nil
}

func listenTonWallet(ctx context.Context, api ton.APIClientWrapped, wallet models.CryptoWallet) error {
	fmt.Println("LISTENING ON TON WALLET")
	asqClient := asynq.NewClient(asynq.RedisClientOpt{Addr: queue.RedisAddr})
	defer asqClient.Close()

	master, err := api.CurrentMasterchainInfo(ctx) // we fetch block just to trigger chain proof check
	if err != nil {
		log.Fatalln("get masterchain info err: ", err.Error())
		return err
	}
	// address on which we are accepting payments
	treasuryAddress := address.MustParseAddr(wallet.Addr)

	_, err = api.GetAccount(ctx, master, treasuryAddress)
	if err != nil {
		log.Fatalln("get masterchain info err: ", err.Error())
		return err
	}

	db := database.GetDB()

	var lastTx models.Transaction
	db.Order("created_at desc").Last(&lastTx, "src_addr = ?", wallet.Addr)

	// Cursor of processed transaction, save it to your db
	// We start from last transaction, will not process transactions older than we started from.
	// After each processed transaction, save lt to your db, to continue after restart
	lastProcessedLT := lastTx.LT
	// channel with new transactions
	transactions := make(chan *tlb.Transaction)

	// it is a blocking call, so we start it asynchronously
	go api.SubscribeOnTransactions(ctx, treasuryAddress, lastProcessedLT, transactions)

	log.Println("waiting for transfers...")

	// USDT master contract addr, but can be any jetton
	usdt_addr := "EQCxE6mUtQJKFnGfaROTKOt1lZbDiiX1kCixRv7Nw2Id_sDs"
	if os.Getenv("env") != "prod" {
		usdt_addr = "kQD0GKBM8ZbryVk2aESmzfU6b9b_8era_IkvBSELujFZPsyy"
	}
	usdt := jetton.NewJettonMasterClient(api, address.MustParseAddr(usdt_addr))
	var dbUsdt models.Coin
	if res := db.First(&dbUsdt, "unit = ? and network = ?", "USDT", "TON"); res.Error != nil {
		log.Fatalln("Unable to find USDT coin in database err:", res.Error)
		return res.Error
	}

	var dbTON models.Coin
	if res := db.First(&dbTON, "unit = ? and network = ?", "TON", "TON"); res.Error != nil {
		log.Fatalln("Unable to find TON coin in database err:", res.Error)
		return res.Error
	}

	// get our jetton wallet address
	treasuryJettonWallet, err := usdt.GetJettonWalletAtBlock(context.Background(), treasuryAddress, master)
	if err != nil {
		log.Fatalln("get jetton wallet address err: ", err.Error())
		return err
	}

	select {
	case <-ctx.Done():
		fmt.Println("LISTENER DONE")
		return nil
	default:
		// listen for new transactions from channel
		for tx := range transactions {
			// only internal messages can increase the balance
			if tx.IO.In != nil && tx.IO.In.MsgType == tlb.MsgTypeInternal {
				ti := tx.IO.In.AsInternal()
				src := ti.SrcAddr

				if dsc, ok := tx.Description.(tlb.TransactionDescriptionOrdinary); ok && dsc.BouncePhase != nil {
					// transaction was bounced, and coins was returned to sender
					// this can happen mostly on custom contracts
					continue
				}

				// verify that event sender is our jetton wallet
				if ti.SrcAddr.Equals(treasuryJettonWallet.Address()) {
					var transfer jetton.TransferNotification
					if err = tlb.LoadFromCell(&transfer, ti.Body.BeginParse()); err == nil {
						memo, _ := transfer.ForwardPayload.BeginParse().LoadStringSnake()
						src = transfer.Sender

						dbTx := models.Transaction{
							SrcAddr: transfer.Sender.String(),
							DstAddr: wallet.Addr,
							Amount:  transfer.Amount.Nano().Uint64(),
							TxHash:  fmt.Sprintf("%q\n", string(tx.Hash)),
							Memo:    memo,
							CoinID:  dbUsdt.ID,
							LT:      tx.LT,
						}

						if res := db.Save(&dbTx); res.Error != nil {
							log.Fatalln("Unable to record transaction err:", res.Error)
							return res.Error
						}
						task, err := tasks.NewTxDeliveryTask(dbTx, wallet)
						if err != nil {
							log.Fatalln("Unable to enqueue transaction err:", err)
							return err
						}
						info, err := asqClient.Enqueue(task)
						if err != nil {
							log.Fatalf("could not enqueue task: %v", err)
						}
						log.Printf("enqueued task: id=%s queue=%s", info.ID, info.Queue)
						log.Println("received", transfer.Amount.Nano(), "USDT from", src.String(), " With memo: ", memo)
					}
				} else {
					if ti.Amount.Nano().Sign() > 0 {
						// show received ton amount
						log.Println("received", ti.Amount.String(), "TON from", src.String(), "with memo", ti.Comment())
					}

					dbTx := models.Transaction{
						SrcAddr: src.String(),
						DstAddr: wallet.Addr,
						Amount:  ti.Amount.Nano().Uint64(),
						TxHash:  fmt.Sprintf("%q\n", string(tx.Hash)),
						Memo:    ti.Comment(),
						CoinID:  dbTON.ID,
						LT:      tx.LT,
					}

					if res := db.Save(&dbTx); res.Error != nil {
						log.Fatalln("Unable to record transaction err:", res.Error)
						return res.Error
					}
				}
			}

			// update last processed lt and save it in db
			lastProcessedLT = tx.LT
			fmt.Println("Last Processed LT: ", lastProcessedLT)
		}

		// it can happen due to none of available liteservers know old enough state for our address
		// (when our unprocessed transactions are too old)
		log.Println("something went wrong, transaction listening unexpectedly finished")
		return fmt.Errorf("something went wrong, transaction listening unexpectedly finished")
	}
	return nil
}
