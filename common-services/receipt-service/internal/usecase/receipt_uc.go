package usecase

import (
	"context"
	"fmt"
	"time"

	"receipt-service/internal/domain"
	"receipt-service/internal/repository"
	"receipt-service/pkg/generator"

	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"

	notificationclient "x/shared/notification"
)

type ReceiptUsecase struct {
    repo repository.ReceiptRepository
    gen  *generator.Generator
    notificationClient *notificationclient.NotificationService
    kafkaWriter *kafka.Writer
}

func NewReceiptUsecase(
    r repository.ReceiptRepository,
    gen *generator.Generator,
    notificationClient *notificationclient.NotificationService,
    kafkaWriter *kafka.Writer,
) *ReceiptUsecase {
    return &ReceiptUsecase{
        repo: r,
        gen:  gen,
        notificationClient: notificationClient,
        kafkaWriter: kafkaWriter,
    }
}

// CreateReceipt generates code and publishes to Kafka
func (uc *ReceiptUsecase) CreateReceipt(ctx context.Context, rec *domain.Receipt) (*domain.Receipt, error) {
    // checkFunc for uniqueness
    checkFunc := func(code string) bool {
        exists, _ := uc.repo.ExistsByCode(ctx, code)
        return exists
    }

    // generate unique code
    var err error
    rec.Code, err = uc.gen.GenerateUnique(checkFunc)
    if err != nil {
        return nil, fmt.Errorf("failed to generate unique receipt code: %w", err)
    }

    rec.Status = "pending"
    rec.CreatedAt = time.Now()

    // map domain → proto
    pb := rec.ToProto()

    // serialize
    data, err := proto.Marshal(pb)
    if err != nil {
        return nil, err
    }

    // publish to Kafka
    err = uc.kafkaWriter.WriteMessages(ctx, kafka.Message{
        Key:   []byte(rec.Code),
        Value: data,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to publish receipt to Kafka: %w", err)
    }

    return rec, nil
}
