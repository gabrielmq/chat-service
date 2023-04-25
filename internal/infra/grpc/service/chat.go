package service

import (
	"github.com/gabrielmq/chat-service/internal/infra/grpc/pb"
	"github.com/gabrielmq/chat-service/internal/usecase/chatcompletionstream"
)

type ChatService struct {
	pb.UnimplementedChatServiceServer
	ChatCompletionStreamUseCase chatcompletionstream.ChatCompletionUseCase
	ChatConfigStream            chatcompletionstream.ChatCompletionConfigurationInput
	StreamChannel               chan chatcompletionstream.ChatCompletionOutput
}

func NewChatService(chatCompletionStreamUseCase chatcompletionstream.ChatCompletionUseCase, chatConfigStream chatcompletionstream.ChatCompletionConfigurationInput, streamChannel chan chatcompletionstream.ChatCompletionOutput) *ChatService {
	return &ChatService{
		ChatCompletionStreamUseCase: chatCompletionStreamUseCase,
		ChatConfigStream:            chatConfigStream,
		StreamChannel:               streamChannel,
	}
}

func (c *ChatService) ChatStream(req *pb.ChatRequest, stream pb.ChatService_ChatStreamServer) error {
	chatConfig := chatcompletionstream.ChatCompletionConfigurationInput{
		Model:                c.ChatConfigStream.Model,
		ModelMaxTokens:       c.ChatConfigStream.ModelMaxTokens,
		Temperature:          c.ChatConfigStream.Temperature,
		TopP:                 c.ChatConfigStream.TopP,
		N:                    c.ChatConfigStream.N,
		Stop:                 c.ChatConfigStream.Stop,
		MaxTokens:            c.ChatConfigStream.MaxTokens,
		InitialSystemMessage: c.ChatConfigStream.InitialSystemMessage,
	}
	input := chatcompletionstream.ChatCompletionInput{
		UserMessage: req.GetUserMessage(),
		UserID:      req.GetUserId(),
		ChatID:      req.GetChatId(),
		Config:      chatConfig,
	}

	ctx := stream.Context()
	go func() {
		for msg := range c.StreamChannel {
			stream.Send(&pb.ChatResponse{
				ChatId:  msg.ChatID,
				UserId:  msg.UserID,
				Content: msg.Content,
			})
		}
	}()

	_, err := c.ChatCompletionStreamUseCase.Execute(ctx, input)
	if err != nil {
		return err
	}

	return nil
}
