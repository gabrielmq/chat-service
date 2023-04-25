package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/gabrielmq/chat-service/internal/domain/entity"
	"github.com/gabrielmq/chat-service/internal/infra/db"
)

type ChatRepositoryMySQL struct {
	DB      *sql.DB
	Queries *db.Queries
}

func NewChatRepositoryMySQL(dbt *sql.DB) *ChatRepositoryMySQL {
	return &ChatRepositoryMySQL{
		DB:      dbt,
		Queries: db.New(dbt),
	}
}

func (c *ChatRepositoryMySQL) Create(ctx context.Context, chat *entity.Chat) error {
	err := c.Queries.Create(
		ctx,
		db.CreateParams{
			ID:               chat.ID,
			UserID:           chat.UserID,
			InitialMessageID: chat.InitialSystemMessage.ID,
			Status:           chat.Status,
			TokenUsage:       int32(chat.TokenUsage),
			Model:            chat.Configuration.Model.GetName(),
			ModelMaxTokens:   int32(chat.Configuration.MaxTokens),
			Temperature:      float64(chat.Configuration.Temperature),
			TopP:             float64(chat.Configuration.TopP),
			N:                int32(chat.Configuration.N),
			Stop:             chat.Configuration.Stop[0],
			MaxTokens:        int32(chat.Configuration.MaxTokens),
			PresencePenalty:  float64(chat.Configuration.FrequencyPenalty),
			FrequencyPenalty: float64(chat.Configuration.FrequencyPenalty),
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		},
	)
	if err != nil {
		return err
	}

	err = c.Queries.AddMessage(
		ctx,
		db.AddMessageParams{
			ID:        chat.InitialSystemMessage.ID,
			ChatID:    chat.ID,
			Content:   chat.InitialSystemMessage.Content,
			Role:      chat.InitialSystemMessage.Role,
			Tokens:    int32(chat.InitialSystemMessage.Tokens),
			CreatedAt: chat.InitialSystemMessage.CreatedAt,
		},
	)
	if err != nil {
		return err
	}
	return nil
}

func (c *ChatRepositoryMySQL) FindByID(ctx context.Context, chatID string) (*entity.Chat, error) {
	chat := &entity.Chat{}
	chatResult, err := c.Queries.FindByID(ctx, chatID)
	if err != nil {
		return nil, errors.New("chat not found")
	}

	chat.ID = chatResult.ID
	chat.UserID = chatResult.UserID
	chat.Status = chatResult.Status
	chat.TokenUsage = int(chatResult.TokenUsage)
	chat.Configuration = &entity.ChatConfiguration{
		Model: &entity.Model{
			Name:      chatResult.Model,
			MaxTokens: int(chatResult.ModelMaxTokens),
		},
		Temperature:      float32(chatResult.Temperature),
		TopP:             float32(chatResult.TopP),
		N:                int(chatResult.N),
		Stop:             []string{chatResult.Stop},
		MaxTokens:        int(chatResult.MaxTokens),
		PresencePenalty:  float32(chatResult.PresencePenalty),
		FrequencyPenalty: float32(chatResult.FrequencyPenalty),
	}

	messages, err := c.Queries.FindMessagesByChatID(ctx, chatID)
	if err != nil {
		return nil, err
	}

	for _, message := range messages {
		chat.Messages = append(chat.Messages, &entity.Message{
			ID:        message.ID,
			Content:   message.Content,
			Role:      message.Role,
			Tokens:    int(message.Tokens),
			Model:     &entity.Model{Name: message.Model},
			CreatedAt: message.CreatedAt,
		})
	}

	erasedMessages, err := c.Queries.FindErasedMessagesByChatID(ctx, chatID)
	if err != nil {
		return nil, err
	}

	for _, message := range erasedMessages {
		chat.ErasedMessages = append(chat.Messages, &entity.Message{
			ID:        message.ID,
			Content:   message.Content,
			Role:      message.Role,
			Tokens:    int(message.Tokens),
			Model:     &entity.Model{Name: message.Model},
			CreatedAt: message.CreatedAt,
		})
	}
	return chat, nil
}

func (r *ChatRepositoryMySQL) Save(ctx context.Context, chat *entity.Chat) error {
	params := db.SaveParams{
		ID:               chat.ID,
		UserID:           chat.UserID,
		Status:           chat.Status,
		TokenUsage:       int32(chat.TokenUsage),
		Model:            chat.Configuration.Model.Name,
		ModelMaxTokens:   int32(chat.Configuration.Model.MaxTokens),
		Temperature:      float64(chat.Configuration.Temperature),
		TopP:             float64(chat.Configuration.TopP),
		N:                int32(chat.Configuration.N),
		Stop:             chat.Configuration.Stop[0],
		MaxTokens:        int32(chat.Configuration.MaxTokens),
		PresencePenalty:  float64(chat.Configuration.PresencePenalty),
		FrequencyPenalty: float64(chat.Configuration.FrequencyPenalty),
		UpdatedAt:        time.Now(),
	}

	err := r.Queries.Save(
		ctx,
		params,
	)
	if err != nil {
		return err
	}
	// delete messages
	err = r.Queries.DeleteChatMessages(ctx, chat.ID)
	if err != nil {
		return err
	}
	// delete erased messages
	err = r.Queries.DeleteErasedChatMessages(ctx, chat.ID)
	if err != nil {
		return err
	}
	// save messages
	i := 0
	for _, message := range chat.Messages {
		err = r.Queries.AddMessage(
			ctx,
			db.AddMessageParams{
				ID:        message.ID,
				ChatID:    chat.ID,
				Content:   message.Content,
				Role:      message.Role,
				Tokens:    int32(message.Tokens),
				Model:     chat.Configuration.Model.Name,
				CreatedAt: message.CreatedAt,
				OrderMsg:  int32(i),
				Erased:    false,
			},
		)
		if err != nil {
			return err
		}
		i++
	}
	// save erased messages
	i = 0
	for _, message := range chat.ErasedMessages {
		err = r.Queries.AddMessage(
			ctx,
			db.AddMessageParams{
				ID:        message.ID,
				ChatID:    chat.ID,
				Content:   message.Content,
				Role:      message.Role,
				Tokens:    int32(message.Tokens),
				Model:     chat.Configuration.Model.Name,
				CreatedAt: message.CreatedAt,
				OrderMsg:  int32(i),
				Erased:    true,
			},
		)
		if err != nil {
			return err
		}
		i++
	}
	return nil
}
