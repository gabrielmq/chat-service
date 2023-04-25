package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/gabrielmq/chat-service/configs"
	"github.com/gabrielmq/chat-service/internal/infra/grpc/server"
	"github.com/gabrielmq/chat-service/internal/infra/repositories"
	"github.com/gabrielmq/chat-service/internal/infra/web"
	"github.com/gabrielmq/chat-service/internal/infra/web/webserver"
	"github.com/gabrielmq/chat-service/internal/usecase/chatcompletion"
	"github.com/gabrielmq/chat-service/internal/usecase/chatcompletionstream"
	_ "github.com/go-sql-driver/mysql"
	"github.com/sashabaranov/go-openai"
)

func main() {
	configs, err := configs.LoadConfig(".")
	if err != nil {
		panic(err)
	}

	conn, err := sql.Open(configs.DBDriver, fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&multiStatements=true",
		configs.DBUser, configs.DBPassword, configs.DBHost, configs.DBPort, configs.DBName))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	repo := repositories.NewChatRepositoryMySQL(conn)
	client := openai.NewClient(configs.OpenAIApiKey)

	chatConfig := chatcompletion.ChatCompletionConfigurationInput{
		Model:                configs.Model,
		ModelMaxTokens:       configs.ModelMaxTokens,
		Temperature:          float32(configs.Temperature),
		TopP:                 float32(configs.TopP),
		N:                    configs.N,
		Stop:                 configs.Stop,
		MaxTokens:            configs.MaxTokens,
		InitialSystemMessage: configs.InitialChatMessage,
	}

	chatConfigStream := chatcompletionstream.ChatCompletionConfigurationInput{
		Model:                configs.Model,
		ModelMaxTokens:       configs.ModelMaxTokens,
		Temperature:          float32(configs.Temperature),
		TopP:                 float32(configs.TopP),
		N:                    configs.N,
		Stop:                 configs.Stop,
		MaxTokens:            configs.MaxTokens,
		InitialSystemMessage: configs.InitialChatMessage,
	}

	streamChannel := make(chan chatcompletionstream.ChatCompletionOutput)
	usecaseStream := chatcompletionstream.NewChatCompletionUseCase(repo, client, streamChannel)

	log.Println("Starting gRPC server on port " + configs.GRPCServerPort)
	grpcServer := server.NewGRPCServer(
		*usecaseStream,
		chatConfigStream,
		configs.GRPCServerPort,
		configs.AuthToken,
		streamChannel,
	)
	go grpcServer.Start()

	usecase := chatcompletion.NewChatCompletionUseCase(repo, client)
	webserver := webserver.NewWebServer(":" + configs.WebServerPort)
	webserverChatHandler := web.NewWebChatGPTHandler(*usecase, chatConfig, configs.AuthToken)
	webserver.AddHandler("/chat", webserverChatHandler.Handle)

	log.Println("Server running on port " + configs.WebServerPort)
	webserver.Start()
}
