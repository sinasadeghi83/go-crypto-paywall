package listeners

import (
	"context"
	"log"
)

func ListenPayments(ctx context.Context) {
	err := ListenOnTON(ctx)
	if err != nil {
		log.Fatalln("Unable to listen on TON err:", err)
	}
}
