// internal/worker/deposit_monitor.go
package worker

import (
	"context"
	"crypto-service/internal/usecase"
	"time"

	"go.uber.org/zap"
)

type DepositMonitor struct {
	depositUsecase *usecase.DepositUsecase
	logger         *zap.Logger
	stopChan       chan bool
}

func NewDepositMonitor(
	depositUsecase *usecase. DepositUsecase,
	logger *zap.Logger,
) *DepositMonitor {
	return &DepositMonitor{
		depositUsecase: depositUsecase,
		logger:         logger,
		stopChan:       make(chan bool),
	}
}

// Start starts the deposit monitoring worker
func (dm *DepositMonitor) Start(ctx context.Context) {
	dm.logger.Info("Starting deposit monitor worker")
	
	// Monitor TRON deposits every 30 seconds
	tronTicker := time.NewTicker(30 * time.Second)
	defer tronTicker.Stop()
	
	// Process pending deposits every 1 minute
	processTicker := time.NewTicker(1 * time.Minute)
	defer processTicker.Stop()
	
	// Send notifications every 2 minutes
	notifyTicker := time.NewTicker(2 * time.Minute)
	defer notifyTicker.Stop()
	
	for {
		select {
		case <-tronTicker.C:
			// Monitor TRON deposits
			if err := dm.depositUsecase.MonitorDeposits(ctx, "TRON"); err != nil {
				dm.logger.Error("Failed to monitor TRON deposits", zap.Error(err))
			}
			
		case <-processTicker.C:
			// Process pending deposits
			if err := dm.depositUsecase.ProcessPendingDeposits(ctx); err != nil {
				dm.logger.Error("Failed to process pending deposits", zap.Error(err))
			}
			
		case <-notifyTicker.C: 
			// Send notifications
			if err := dm.depositUsecase. NotifyPendingDeposits(ctx); err != nil {
				dm.logger.Error("Failed to send deposit notifications", zap.Error(err))
			}
			
		case <-dm.stopChan:
			dm.logger.Info("Stopping deposit monitor worker")
			return
			
		case <-ctx.Done():
			dm.logger.Info("Context cancelled, stopping deposit monitor")
			return
		}
	}
}

// Stop stops the deposit monitor
func (dm *DepositMonitor) Stop() {
	close(dm.stopChan)
}