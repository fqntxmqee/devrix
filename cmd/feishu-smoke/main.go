package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/channel/adapters"
	"github.com/devrix/devrix/internal/shared/config"
)

func main() {
	userCfg, err := config.LoadUserConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	feishuCfg := &adapters.FeishuConfig{
		AppID:         userCfg.IM.Feishu.AppID,
		AppSecret:     userCfg.IM.Feishu.AppSecret,
		ReactionEmoji: userCfg.IM.Feishu.ReactionEmoji,
		DoneEmoji:     userCfg.IM.Feishu.DoneEmoji,
		ReplyInThread: userCfg.IM.Feishu.IsReplyInThread(),
	}
	if feishuCfg.ReactionEmoji == "" {
		feishuCfg.ReactionEmoji = "OnIt"
	}

	adapter := adapters.NewFeishuAdapter(nil, feishuCfg, config.DefaultConfig())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	chatID := "oc_a444ebe708203fb4d38b18a902ac9859"
	msgID, err := latestUserMessageID(ctx, userCfg.IM.Feishu.AppID, userCfg.IM.Feishu.AppSecret, chatID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "find user message: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("target message: %s\n", msgID)

	if err := adapter.AddReaction(ctx, msgID, feishuCfg.ReactionEmoji); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: typing reaction: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("OK: typing reaction added (%s)\n", feishuCfg.ReactionEmoji)

	replyText := fmt.Sprintf("devrix smoke test %s", time.Now().Format("15:04:05"))
	if err := adapter.ReplyMessage(ctx, msgID, replyText); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: thread reply: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("OK: thread reply sent: %s\n", replyText)

	if feishuCfg.DoneEmoji != "" && feishuCfg.DoneEmoji != "none" {
		if err := adapter.AddReaction(ctx, msgID, feishuCfg.DoneEmoji); err != nil {
			fmt.Fprintf(os.Stderr, "WARN: done emoji: %v\n", err)
		} else {
			fmt.Printf("OK: done reaction added (%s)\n", feishuCfg.DoneEmoji)
		}
	}

	fmt.Println("SMOKE PASS")
}

func latestUserMessageID(ctx context.Context, appID, appSecret, chatID string) (string, error) {
	token, err := tenantToken(ctx, appID, appSecret)
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("https://open.feishu.cn/open-apis/im/v1/messages?container_id_type=chat&container_id=%s&page_size=20&sort_type=ByCreateTimeDesc", chatID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var parsed struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				MessageID string `json:"message_id"`
				Sender    struct {
					SenderType string `json:"sender_type"`
				} `json:"sender"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if parsed.Code != 0 {
		return "", fmt.Errorf("list messages code=%d", parsed.Code)
	}
	for _, item := range parsed.Data.Items {
		if item.Sender.SenderType == "user" {
			return item.MessageID, nil
		}
	}
	return "", fmt.Errorf("no user message found in chat %s", chatID)
}

func tenantToken(ctx context.Context, appID, appSecret string) (string, error) {
	payload := fmt.Sprintf(`{"app_id":"%s","app_secret":"%s"}`, appID, appSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal", strings.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var parsed struct {
		Code              int    `json:"code"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if parsed.Code != 0 || parsed.TenantAccessToken == "" {
		return "", fmt.Errorf("token request failed")
	}
	return parsed.TenantAccessToken, nil
}

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
}
