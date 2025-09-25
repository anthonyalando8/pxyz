package usecase

import (
	"context"
	"fmt"
	"time"

	"receipt-service/internal/domain"
	"receipt-service/internal/repository"
	"receipt-service/pkg/generator"
	"receipt-service/pkg/utils"

	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
	receiptpb "x/shared/genproto/shared/accounting/receipt/v2"

	notificationclient "x/shared/notification"
)

type ReceiptUsecase struct {
    repo repository.ReceiptRepository
    gen  *generator.Generator
	gen2 *receiptutil.ReceiptGenerator
    notificationClient *notificationclient.NotificationService
    kafkaWriter *kafka.Writer
}

func NewReceiptUsecase(
    r repository.ReceiptRepository,
    gen *generator.Generator,
	gen2 *receiptutil.ReceiptGenerator,
    notificationClient *notificationclient.NotificationService,
    kafkaWriter *kafka.Writer,
) *ReceiptUsecase {
    return &ReceiptUsecase{
        repo: r,
        gen:  gen,
		gen2: gen2,
        notificationClient: notificationClient,
        kafkaWriter: kafkaWriter,
    }
}

func (uc *ReceiptUsecase) CreateReceipts(ctx context.Context, recs []*domain.Receipt) ([]*domain.Receipt, error) {
    now := time.Now()

    // 1. Generate codes + prepare records
    for _, rec := range recs {
        var code string

        // Try gen2 first (preferred)
        for i := 0; i < 3; i++ {
            candidate := uc.gen2.GenerateReceiptID(rec.Type)
            if candidate != "" {
                code = candidate
                break
            }
        }

        // Fallback to gen1
        if code == "" {
            var err error
            code, err = uc.gen.GenerateUnique(nil)
            if err != nil {
                return nil, fmt.Errorf("failed to generate unique receipt code: %w", err)
            }
        }

        rec.Code = code
        rec.Status = "pending"
        rec.CreatedAt = now
        rec.UpdatedAt = &now
    }

    // 2. Publish to Kafka immediately
    msgs := make([]kafka.Message, len(recs))
    for i, rec := range recs {
        msg := &receiptpb.ReceiptMessage{
            Type:    receiptpb.ReceiptMessageType_CREATE,
            Receipt: rec.ToProto(),
        }
        data, err := proto.Marshal(msg)
        if err != nil {
            return nil, fmt.Errorf("marshal create message (code=%s): %w", rec.Code, err)
        }
        msgs[i] = kafka.Message{
            Key:   []byte(rec.Code),
            Value: data,
        }
    }

    if err := uc.kafkaWriter.WriteMessages(ctx, msgs...); err != nil {
        return nil, fmt.Errorf("failed to publish receipts to Kafka: %w", err)
    }

    // 3. Return receipts; DB write happens asynchronously in workers
    return recs, nil
}

func (uc *ReceiptUsecase) UpdateReceipts(ctx context.Context, updates []*domain.ReceiptUpdate) error {
    if len(updates) == 0 {
        return nil
    }

    // 1. Publish updates to Kafka only (DB update happens in worker)
    msgs := make([]kafka.Message, len(updates))
    for i, upd := range updates {
        msg := &receiptpb.ReceiptMessage{
            Type:   receiptpb.ReceiptMessageType_UPDATE,
            Update: upd.ToProto(),
        }
        data, err := proto.Marshal(msg)
        if err != nil {
            return fmt.Errorf("marshal update message (code=%s): %w", upd.Code, err)
        }

        msgs[i] = kafka.Message{
            Key:   []byte(upd.Code),
            Value: data,
        }
    }

    if err := uc.kafkaWriter.WriteMessages(ctx, msgs...); err != nil {
        return fmt.Errorf("failed to publish updated receipts to Kafka: %w", err)
    }

    return nil
}



func (uc *ReceiptUsecase) GetReceiptByCode(ctx context.Context, code string) (*domain.Receipt, error) {
	if code == "" {
		return nil, fmt.Errorf("receipt code cannot be empty")
	}

	rec, err := uc.repo.GetByCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("get receipt by code: %w", err)
	}

	return rec, nil
}