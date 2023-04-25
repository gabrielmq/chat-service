package web

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/gabrielmq/chat-service/internal/usecase/chatcompletion"
)

type WebChatGPTHandler struct {
	CompletionUseCase chatcompletion.ChatCompletionUseCase
	Configuration     chatcompletion.ChatCompletionConfigurationInput
	AuthToken         string
}

func NewWebChatGPTHandler(usecase chatcompletion.ChatCompletionUseCase, cfg chatcompletion.ChatCompletionConfigurationInput, authToken string) *WebChatGPTHandler {
	return &WebChatGPTHandler{
		CompletionUseCase: usecase,
		Configuration:     cfg,
		AuthToken:         authToken,
	}
}

func (h *WebChatGPTHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if r.Header.Get("Authorization") != h.AuthToken {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	if !json.Valid(body) {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	var input chatcompletion.ChatCompletionInput
	if err := json.Unmarshal(body, &input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	input.Configuration = h.Configuration

	output, err := h.CompletionUseCase.Execute(r.Context(), input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(output)
}
