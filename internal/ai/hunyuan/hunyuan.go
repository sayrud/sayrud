// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package hunyuan

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/regions"
	hunyuan "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/hunyuan/v20230901"

	"github.com/wuhan005/sayrud/internal/ai"
)

type Client struct {
}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) Advice(ctx context.Context, action ai.ActionType, msg []*ai.Message) (*ai.AIAdvice, error) {
	messages := make([]*hunyuan.Message, 0, len(msg)+1)

	// Set the initial prompt.
	var initialPrompt string
	switch action {
	case ai.ActionTypeTables:
		initialPrompt = string(AdviceTablePrompt)
	default:
		return nil, ai.ErrActionNotFound
	}
	messages = append(messages, &hunyuan.Message{
		Role:    common.StringPtr("system"),
		Content: common.StringPtr(initialPrompt),
	})

	for _, m := range msg {
		if m.Role != "assistant" && m.Role != "user" {
			continue
		}

		messages = append(messages, &hunyuan.Message{
			Role:    common.StringPtr(m.Role),
			Content: common.StringPtr(m.Content),
		})
	}

	output, err := chatCompletions(ctx, messages)
	if err != nil {
		return nil, errors.Wrap(err, "chat completions")
	}

	jsonRe := regexp.MustCompile("```json\\s*([\\s\\S]*?)\\s*```")
	match := jsonRe.FindStringSubmatch(output)
	var jsonStr string
	if len(match) > 1 {
		jsonStr = strings.TrimSpace(match[1])
	}

	var responseData struct {
		Operation string          `json:"operation"`
		Data      json.RawMessage `json:"data"`
	}
	_ = json.Unmarshal([]byte(jsonStr), &responseData)

	descriptionRe := regexp.MustCompile("```json\\s*([\\s\\S]*?)\\s*```([\\s\\S]*)")
	match = descriptionRe.FindStringSubmatch(output)
	var description string
	if len(match) > 2 {
		description = strings.TrimSpace(match[2])
	}

	return &ai.AIAdvice{
		RawContent: output,

		Action:      responseData.Operation,
		ActionJSON:  responseData.Data,
		Description: description,
	}, nil
}

func chatCompletions(ctx context.Context, messages []*hunyuan.Message) (string, error) {
	credential := common.NewCredential(
		os.Getenv("TENCENTCLOUD_SECRET_ID"),
		os.Getenv("TENCENTCLOUD_SECRET_KEY"),
	)

	cpf := profile.NewClientProfile()
	client, err := hunyuan.NewClient(credential, regions.Guangzhou, cpf)
	if err != nil {
		return "", errors.Wrap(err, "new client")
	}

	request := hunyuan.NewChatCompletionsRequest()
	request.SetContext(ctx)
	request.Model = common.StringPtr("hunyuan-lite")
	request.Stream = common.BoolPtr(false)
	request.Messages = messages

	var response hunyuan.ChatCompletionsResponse
	if err := client.Send(request, &response); err != nil {
		return "", errors.Wrap(err, "send")
	}

	var result string
	choices := response.Response.Choices
	for _, choice := range choices {
		result += *choice.Message.Content
	}
	return result, nil
}
