package chatcompletion

import (
	"context"
	"errors"

	"github.com/gabrielmq/chat-service/internal/domain/entity"
	"github.com/gabrielmq/chat-service/internal/domain/gateway"
	"github.com/sashabaranov/go-openai"
)

type ChatCompletionConfigurationInput struct {
	Model                string
	ModelMaxTokens       int
	Temperature          float32  // 0.0 to 1.0
	TopP                 float32  // 0.0 to 1.0 - to a low value, like 0.1, the model will be very conservative in its word choices, and will tend to generate relatively predictable prompts
	N                    int      // number of messages to generate
	Stop                 []string // list of tokens to stop on
	MaxTokens            int      // number of tokens to generate
	PresencePenalty      float32  // -2.0 to 2.0 - Number between -2.0 and 2.0. Positive values penalize new tokens based on whether they appear in the text so far, increasing the model's likelihood to talk about new topics.
	FrequencyPenalty     float32  // -2.0 to 2.0 - Number between -2.0 and 2.0. Positive values penalize new tokens based on their existing frequency in the text so far, increasing the model's likelihood to talk about new topics.
	InitialSystemMessage string
}

type ChatCompletionInput struct {
	ChatID        string                           `json:"chat_id,omitempty"`
	UserID        string                           `json:"user_id"`
	UserMessage   string                           `json:"user_message"`
	Configuration ChatCompletionConfigurationInput `json:"Configuration"`
}

type ChatCompletionOutput struct {
	ChatID  string `json:"chat_id"`
	UserID  string `json:"user_id"`
	Content string `json:"content"`
}

type ChatCompletionUseCase struct {
	ChatGateway  gateway.ChatGateway
	OpenAIClient *openai.Client
}

func NewChatCompletionUseCase(chatGateway gateway.ChatGateway, openAIClient *openai.Client) *ChatCompletionUseCase {
	return &ChatCompletionUseCase{
		ChatGateway:  chatGateway,
		OpenAIClient: openAIClient,
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
			err = uc.ChatGateway.Create(ctx, chat)
			if err != nil {
				return nil, errors.New("error persisting new chat: " + err.Error())
			}
		} else {
			return nil, errors.New("error fetching existing chat: " + err.Error())
		}
	}

	userMessage, err := entity.NewMessage("user", input.UserMessage, chat.Configuration.Model)
	if err != nil {
		return nil, errors.New("error creating new message: " + err.Error())
	}
	err = chat.AddMessage(userMessage)
	if err != nil {
		return nil, errors.New("error adding new message: " + err.Error())
	}

	messages := []openai.ChatCompletionMessage{}
	for _, msg := range chat.Messages {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	resp, err := uc.OpenAIClient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:            chat.Configuration.Model.Name,
			Messages:         messages,
			MaxTokens:        chat.Configuration.MaxTokens,
			Temperature:      chat.Configuration.Temperature,
			TopP:             chat.Configuration.TopP,
			PresencePenalty:  chat.Configuration.PresencePenalty,
			FrequencyPenalty: chat.Configuration.FrequencyPenalty,
			Stop:             chat.Configuration.Stop,
		},
	)
	if err != nil {
		return nil, errors.New("error openai: " + err.Error())
	}

	assistant, err := entity.NewMessage("assistant", resp.Choices[0].Message.Content, chat.Configuration.Model)
	if err != nil {
		return nil, err
	}
	err = chat.AddMessage(assistant)
	if err != nil {
		return nil, err
	}

	err = uc.ChatGateway.Save(ctx, chat)
	if err != nil {
		return nil, err
	}

	output := &ChatCompletionOutput{
		ChatID:  chat.ID,
		UserID:  input.UserID,
		Content: resp.Choices[0].Message.Content,
	}

	return output, nil
}

func createNewChat(input ChatCompletionInput) (*entity.Chat, error) {
	model := entity.NewModel(input.Configuration.Model, input.Configuration.ModelMaxTokens)
	chatConfiguration := &entity.ChatConfiguration{
		Temperature:      input.Configuration.Temperature,
		TopP:             input.Configuration.TopP,
		N:                input.Configuration.N,
		Stop:             input.Configuration.Stop,
		MaxTokens:        input.Configuration.MaxTokens,
		PresencePenalty:  input.Configuration.PresencePenalty,
		FrequencyPenalty: input.Configuration.FrequencyPenalty,
		Model:            model,
	}

	initialMessage, err := entity.NewMessage("system", input.Configuration.InitialSystemMessage, model)
	if err != nil {
		return nil, errors.New("error creating initial message: " + err.Error())
	}
	chat, err := entity.NewChat(input.UserID, initialMessage, chatConfiguration)
	if err != nil {
		return nil, errors.New("error creating new chat: " + err.Error())
	}
	return chat, nil
}
