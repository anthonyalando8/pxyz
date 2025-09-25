package service

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"

	"receipt-service/internal/domain"
	"receipt-service/internal/repository"
	receiptpb "x/shared/genproto/shared/accounting/receipt/v2"
)
func StartReceiptWorker(ctx context.Context, brokers []string, repo repository.ReceiptRepository) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   "receipts",
		GroupID: "receipt-workers",
	})

	const (
		createBatchSize = 500     // large batch for high throughput
		updateBatchSize = 500
		flushInterval   = 500 * time.Millisecond
	)

	var (
		createBatch []*domain.Receipt
		updateBatch []*domain.ReceiptUpdate
		lastFlush   = time.Now()
		mu          sync.Mutex
	)

	flush := func() {
		mu.Lock()
		defer mu.Unlock()

		if len(createBatch) > 0 {
			batchPtrs := make([]*domain.Receipt, len(createBatch))
			copy(batchPtrs, createBatch)
			if err := repo.CreateBatch(ctx, batchPtrs, nil); err != nil {
				log.Printf("[ERROR] create batch failed: %v", err)
			} else {
				log.Printf("[DEBUG] flushed CREATE batch of %d receipts", len(createBatch))
			}
			createBatch = createBatch[:0]
		}

		if len(updateBatch) > 0 {
			if _, err := repo.UpdateBatch(ctx, updateBatch); err != nil {
				log.Printf("[ERROR] update batch failed: %v", err)
			} else {
				log.Printf("[DEBUG] flushed UPDATE batch of %d receipts", len(updateBatch))
			}
			updateBatch = updateBatch[:0]
		}

		lastFlush = time.Now()
	}

	// ticker for periodic flush
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				flush()
				return
			case <-ticker.C:
				if time.Since(lastFlush) >= flushInterval {
					flush()
				}
			}
		}
	}()

	for {
		m, err := r.ReadMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				flush()
				return
			}
			log.Printf("[ERROR] kafka read error: %v", err)
			continue
		}

		var msg receiptpb.ReceiptMessage
		if err := proto.Unmarshal(m.Value, &msg); err != nil {
			log.Printf("[ERROR] failed to unmarshal ReceiptMessage: %v", err)
			continue
		}

		mu.Lock()
		switch msg.Type {
		case receiptpb.ReceiptMessageType_CREATE:
			r := domain.ReceiptFromProto(msg.Receipt) // returns value
			createBatch = append(createBatch, &r)     // append pointer
			if len(createBatch) >= createBatchSize {
				mu.Unlock()
				flush()
				continue
			}

		case receiptpb.ReceiptMessageType_UPDATE:
			updateBatch = append(updateBatch, domain.ReceiptUpdateFromProto(msg.Update))
			if len(updateBatch) >= updateBatchSize {
				mu.Unlock()
				flush()
				continue
			}

		default:
			log.Printf("[WARN] unknown ReceiptMessageType: %v", msg.Type)
		}
		mu.Unlock()
	}
}
