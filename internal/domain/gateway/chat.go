package gateway

import (
	"context"

	"github.com/gabrielmq/chat-service/internal/domain/entity"
)

type ChatGateway interface {
	Create(ctx context.Context, chat *entity.Chat) error
	FindByID(ctx context.Context, chatID string) (*entity.Chat, error)
	Save(ctx context.Context, chat *entity.Chat) error
}
