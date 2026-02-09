// internal/worker/sweep_worker.go
package worker

import (
	"context"
	"crypto-service/internal/usecase"
	"time"

	"go.uber.org/zap"
)

type SweepWorker struct {
	transactionUsecase *usecase.TransactionUsecase
	logger             *zap. Logger
	stopChan           chan bool
}

func NewSweepWorker(
	transactionUsecase *usecase.TransactionUsecase,
	logger *zap.Logger,
) *SweepWorker {
	return &SweepWorker{
		transactionUsecase:  transactionUsecase,
		logger:             logger,
		stopChan:           make(chan bool),
	}
}

func (sw *SweepWorker) Start(ctx context.Context) {
	sw.logger.Info("Starting sweep worker")
	
	// Sweep every 6 hours
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			sw.sweepAll(ctx)
			
		case <-sw.stopChan:
			sw.logger.Info("Stopping sweep worker")
			return
			
		case <-ctx.Done():
			sw.logger.Info("Context cancelled, stopping sweep worker")
			return
		}
	}
}

func (sw *SweepWorker) sweepAll(ctx context.Context) {
	sw.logger.Info("Starting scheduled sweep")
	
	// Sweep TRON assets
	sw.sweepChain(ctx, "TRON", []string{"TRX", "USDT"})
	
	// Sweep Bitcoin
	sw.sweepChain(ctx, "BITCOIN", []string{"BTC"})
}

func (sw *SweepWorker) sweepChain(ctx context.Context, chain string, assets []string) {
	for _, asset := range assets {
		
		_, err := sw.transactionUsecase.SweepAllUsers(ctx, chain, asset)
		if err != nil {
			sw.logger.Error("Sweep failed",
				zap. Error(err),
				zap.String("chain", chain),
				zap.String("asset", asset))
		}
	}
}

func (sw *SweepWorker) Stop() {
	close(sw.stopChan)
}