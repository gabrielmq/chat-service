package chatcompletionstream

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/gabrielmq/chat-service/internal/domain/entity"
	"github.com/gabrielmq/chat-service/internal/domain/gateway"
	"github.com/sashabaranov/go-openai"
)

type ChatCompletionConfigurationInput struct {
	Model                string
	ModelMaxTokens       int
	Temperature          float32
	TopP                 float32
	N                    int
	Stop                 []string
	MaxTokens            int
	PresencePenalty      float32
	FrequencyPenalty     float32
	InitialSystemMessage string
}

type ChatCompletionInput struct {
	ChatID      string
	UserID      string
	UserMessage string
	Config      ChatCompletionConfigurationInput
}

type ChatCompletionOutput struct {
	ChatID  string
	UserID  string
	Content string
}

type ChatCompletionUseCase struct {
	ChatGateway  gateway.ChatGateway
	OpenAiClient *openai.Client
	Stream       chan ChatCompletionOutput
}

func NewChatCompletionUseCase(chatGateway gateway.ChatGateway, openAiClient *openai.Client, stream chan ChatCompletionOutput) *ChatCompletionUseCase {
	return &ChatCompletionUseCase{
		ChatGateway:  chatGateway,
		OpenAiClient: openAiClient,
		Stream:       stream,
	}
}

func (uc *ChatCompletionUseCase) Execute(ctx context.Context, input ChatCompletionInput) (*ChatCompletionOutput, error) {
	chat, err := uc.ChatGateway.FindByID(ctx, input.ChatID)
	if err != nil {
		if err.Error() == "chat not found" {
			chat, err = createNewChat(input)
			if err != nil {
				return nil, errors.New("error creating new chat: " + err.Error())
			}

			if err = uc.ChatGateway.Create(ctx, chat); err != nil {
				return nil, errors.New("error persisting new chat: " + err.Error())
			}
		} else {
			return nil, errors.New("error fetching existing chat: " + err.Error())
		}
	}

	userMessage, err := entity.NewMessage("user", input.UserMessage, chat.Configuration.Model)
	if err != nil {
		return nil, errors.New("error creating user message: " + err.Error())
	}

	if err := chat.AddMessage(userMessage); err != nil {
		return nil, errors.New("error adding new message: " + err.Error())
	}

	messages := []openai.ChatCompletionMessage{}
	for _, message := range chat.Messages {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    message.Role,
			Content: message.Content,
		})
	}

	res, err := uc.OpenAiClient.CreateChatCompletionStream(
		ctx,
		openai.ChatCompletionRequest{
			Model:            chat.Configuration.Model.Name,
			Messages:         messages,
			MaxTokens:        chat.Configuration.MaxTokens,
			Temperature:      chat.Configuration.Temperature,
			TopP:             chat.Configuration.TopP,
			PresencePenalty:  chat.Configuration.PresencePenalty,
			FrequencyPenalty: chat.Configuration.FrequencyPenalty,
			Stop:             chat.Configuration.Stop,
			Stream:           true,
		},
	)
	if err != nil {
		return nil, errors.New("error creating chat completion: " + err.Error())
	}

	var fullResponse strings.Builder
	for {
		response, err := res.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, errors.New("error streaming response: " + err.Error())
		}

		fullResponse.WriteString(response.Choices[0].Delta.Content)

		uc.Stream <- ChatCompletionOutput{
			ChatID:  chat.ID,
			UserID:  input.UserID,
			Content: fullResponse.String(),
		}
	}

	assistant, err := entity.NewMessage("assistant", fullResponse.String(), chat.Configuration.Model)
	if err != nil {
		return nil, errors.New("error creating assistant message: " + err.Error())
	}

	if err := chat.AddMessage(assistant); err != nil {
		return nil, errors.New("error adding new message: " + err.Error())
	}

	if err := uc.ChatGateway.Save(ctx, chat); err != nil {
		return nil, errors.New("error saving chat: " + err.Error())
	}
	return &ChatCompletionOutput{
		ChatID:  chat.ID,
		UserID:  input.UserID,
		Content: fullResponse.String(),
	}, nil
}

func createNewChat(input ChatCompletionInput) (*entity.Chat, error) {
	model := entity.NewModel(input.Config.Model, input.Config.ModelMaxTokens)
	chatConfig := &entity.ChatConfiguration{
		Temperature:      input.Config.Temperature,
		TopP:             input.Config.TopP,
		N:                input.Config.N,
		Stop:             input.Config.Stop,
		MaxTokens:        input.Config.MaxTokens,
		PresencePenalty:  input.Config.PresencePenalty,
		FrequencyPenalty: input.Config.FrequencyPenalty,
		Model:            model,
	}
	initialMessage, err := entity.NewMessage("system", input.Config.InitialSystemMessage, model)
	if err != nil {
		return nil, errors.New("error creating initial message: " + err.Error())
	}

	chat, err := entity.NewChat(input.UserID, initialMessage, chatConfig)
	if err != nil {
		return nil, errors.New("error creating new chat: " + err.Error())
	}
	return chat, nil
}
