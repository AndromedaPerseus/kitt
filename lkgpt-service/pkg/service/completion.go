package service

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/livekit/protocol/logger"
	openai "github.com/sashabaranov/go-openai"
)

var (
	ErrAlreadyUsed = errors.New("chat completion already used")
)

type ChatCompletion struct {
	client   *openai.Client
	language string
}

func NewChatCompletion(client *openai.Client, language string) *ChatCompletion {
	return &ChatCompletion{
		client:   client,
		language: language,
	}
}

func (c *ChatCompletion) Complete(ctx context.Context, history []*Sentence, prompt string) (*ChatStream, error) {
	var sb strings.Builder
	for _, s := range history {
		sb.WriteString(s.Name)
		sb.WriteString(": ")
		sb.WriteString(s.Transcript)
		sb.WriteString("\n")
	}
	conversation := sb.String()
	stream, err := c.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleSystem,
				Content: "You are a voice assistant named LiveGPT." +
					"Answer with multiple small/medium sentences with the right punctuation. Only use dot (.) to end a sentence" +
					"Here is the current conversation, the name of the user is prefixed to each message." +
					"Answer the user question with the " + LanguageCode + " language.",
			},
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: conversation,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Stream: true,
	})

	if err != nil {
		logger.Errorw("error creating chat completion stream", err)
		return nil, err
	}

	return &ChatStream{
		stream: stream,
	}, nil
}

// Wrapper around openai.ChatCompletionStream to return only complete sentences
type ChatStream struct {
	stream *openai.ChatCompletionStream
}

func (c *ChatStream) Recv() (string, error) {
	sb := strings.Builder{}
	for {
		response, err := c.stream.Recv()
		if err != nil {
			content := sb.String()
			if err == io.EOF && len(strings.TrimSpace(content)) != 0 {
				return content, nil
			}
			return "", err
		}

		delta := response.Choices[0].Delta.Content
		sb.WriteString(delta)

		if strings.HasSuffix(strings.TrimSpace(delta), ".") {
			return sb.String(), nil
		}
	}
}

func (c *ChatStream) Close() {
	c.stream.Close()
}
