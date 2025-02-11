package queue

import (
	"context"
	"log"

	"github.com/hibiken/asynq"
	"github.com/sinasadeghi83/go-paywall/tasks"
)

var RedisAddr string

func Setup(ctx context.Context, redisAddr string, userHandler tasks.HandlePaid) {
	RedisAddr = redisAddr
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: RedisAddr},
		asynq.Config{
			// Specify how many concurrent workers to use
			Concurrency: 10,
			// Optionally specify multiple queues with different priority.
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
			// See the godoc for other configuration options
		},
	)

	// mux maps a type to a handler
	mux := asynq.NewServeMux()
	mux.HandleFunc(tasks.TypeTxDelivery, tasks.HandleTxDeliveryTask(userHandler))

	go runServer(srv, mux)
}

func runServer(srv *asynq.Server, mux *asynq.ServeMux) {
	if err := srv.Run(mux); err != nil {
		log.Fatalf("could not run server: %v", err)
	}
}
