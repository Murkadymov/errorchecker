package main

import (
	"context"
	"errorchecker/config"
	"errorchecker/internal/entity/errorchecker"
	"errorchecker/internal/httpclient"
	logger2 "errorchecker/internal/pkg/logger"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {

	log := logger2.NewLogger()

	cfg := config.MustLoad(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	wg := &sync.WaitGroup{}

	httpClient := httpclient.NewErrorChecker(
		os.Getenv("HOST"),
		cfg.WB.Cluster,
		"",
		cfg.Band.BandURL,
		os.Getenv("BAND_WEBHOOK_ENDPOINT"),
	)

	headers := errorchecker.NewHeadersStorage(os.Getenv("COOKIE_SECRET"))
	httpClient.RunRequests(ctx, cfg.WB.Interval, headers, log, wg)

	<-ctx.Done()

	wg.Wait()

	fmt.Println("received signal and read from ctx.Done")
	fmt.Println("checker stopped")
}
