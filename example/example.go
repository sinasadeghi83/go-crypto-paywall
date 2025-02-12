package example

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	database "github.com/sinasadeghi83/go-crypto-paywall/db"
	"github.com/sinasadeghi83/go-crypto-paywall/listeners"
	"github.com/sinasadeghi83/go-crypto-paywall/models"
	"github.com/sinasadeghi83/go-crypto-paywall/queue"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const redisAddr = "127.0.0.1:6379"

func main() {
	godotenv.Load(".env")

	ctx := context.Background()

	// Step 1: Setup database
	setupDB()

	// Step 2: Setup queue
	queue.Setup(ctx, redisAddr, func(t models.Transaction, i models.Invoice, cw models.CryptoWallet) {
		fmt.Println("Invoice paid:", i)
	})
	var w models.CryptoWallet
	database.GetDB().First(&w, "status = 'active' and network = 'TON'")
	fmt.Println("TON URL:", w.TokenTonURL("1100000", "trump", "kQD0GKBM8ZbryVk2aESmzfU6b9b_8era_IkvBSELujFZPsyy"))

	// Step3: listen for payments
	listeners.ListenPayments(ctx)
}

func setupDB() *gorm.DB {
	user := os.Getenv("MYSQL_USER")
	pass := os.Getenv("MYSQL_PASS")
	host := os.Getenv("MYSQL_HOST")
	port := os.Getenv("MYSQL_PORT")
	dbname := os.Getenv("MYSQL_DB")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", user, pass, host, port, dbname)

	dbInstance, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalln("Cannot connect to database. Err: ", err)
	}

	// Configure the database connection pool
	sqlDB, err := dbInstance.DB()
	if err != nil {
		log.Fatal("Failed to get sql.DB:", err)
	}

	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	database.Setup(dbInstance)
	return dbInstance
}
