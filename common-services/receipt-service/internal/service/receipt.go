package service

import (
	"log"
	"context"
	"receipt-service/internal/domain"
	"receipt-service/internal/repository"

	"github.com/segmentio/kafka-go"
    receiptpb "x/shared/genproto/shared/accounting/receipt/v2"
    "google.golang.org/protobuf/proto"
)

func StartReceiptWorker(ctx context.Context, brokers []string, repo repository.ReceiptRepository) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   "receipts",
		GroupID: "receipt-workers",
	})

	var batch []domain.Receipt
	for {
		m, err := r.ReadMessage(ctx)
		if err != nil {
			log.Printf("error reading kafka message: %v", err)
			continue
		}

		var pb receiptpb.Receipt
		if err := proto.Unmarshal(m.Value, &pb); err != nil {
			log.Printf("unmarshal error: %v", err)
			continue
		}

		receipt := domain.ReceiptFromProto(&pb)
		batch = append(batch, receipt)

		if len(batch) >= 100 {
			// Convert []domain.Receipt to []*domain.Receipt
			batchPtrs := make([]*domain.Receipt, len(batch))
			for i := range batch {
				batchPtrs[i] = &batch[i]
			}
			err := repo.CreateBatch(ctx, batchPtrs, nil)
			if err != nil {
				log.Printf("db insert error: %v", err)
				// optional: DLQ
			}
			batch = batch[:0]
		}
	}
}
