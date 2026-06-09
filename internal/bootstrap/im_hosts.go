package bootstrap

import (
	"context"
	"log/slog"
	"strings"

	"github.com/devrix/devrix/internal/layers/communication/adapters"
	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/shared/config"
)

// IMHosts holds started IM adapters for the enabled provider in user config.
type IMHosts struct {
	Feishu   *adapters.FeishuAdapter
	DingTalk *adapters.DingTalkAdapter
}

func (h *IMHosts) Active() bool {
	if h == nil {
		return false
	}
	return h.Feishu != nil || h.DingTalk != nil
}

// Stop stops all started IM adapters.
func (h *IMHosts) Stop() {
	if h == nil {
		return
	}
	if h.Feishu != nil {
		_ = h.Feishu.Stop()
	}
	if h.DingTalk != nil {
		_ = h.DingTalk.Stop()
	}
}

// WireIM prepares IM adapters from user config. Returns the gateway event handler
// (adapter when IM is enabled, otherwise defaultHandler).
func WireIM(
	userCfg *config.UserConfig,
	commCfg *config.CommunicationConfig,
	defaultHandler gateway.EventHandler,
) (*IMHosts, gateway.EventHandler) {
	if userCfg == nil || !userCfg.IM.Enabled {
		return nil, defaultHandler
	}

	hosts := &IMHosts{}
	provider := stringsTrim(userCfg.IM.Platform.Provider)

	switch provider {
	case "feishu":
		if userCfg.IM.Feishu.AppID == "" || userCfg.IM.Feishu.AppSecret == "" {
			slog.Warn("im.feishu enabled but app_id/app_secret missing; skipping feishu adapter")
			return nil, defaultHandler
		}
		feishuCfg := &adapters.FeishuConfig{
			AppID:         userCfg.IM.Feishu.AppID,
			AppSecret:     userCfg.IM.Feishu.AppSecret,
			BotName:       userCfg.IM.Feishu.BotName,
			Domain:        userCfg.IM.Feishu.Domain,
			EncryptKey:    userCfg.IM.Feishu.EncryptKey,
			CallbackPath:  "/feishu/webhook",
			Port:          "8080",
			UseWebhook:    userCfg.IM.Feishu.UseWebhook,
			ReactionEmoji: userCfg.IM.Feishu.ReactionEmoji,
			DoneEmoji:     userCfg.IM.Feishu.DoneEmoji,
			ReplyInThread: userCfg.IM.Feishu.IsReplyInThread(),
			ProgressStyle: userCfg.IM.Feishu.ProgressStyle,
		}
		hosts.Feishu = adapters.NewFeishuAdapter(nil, feishuCfg, commCfg)
		return hosts, hosts.Feishu

	case "dingtalk":
		if userCfg.IM.DingTalk.AppKey == "" || userCfg.IM.DingTalk.AppSecret == "" {
			slog.Warn("im.dingtalk enabled but app_key/app_secret missing; skipping dingtalk adapter")
			return nil, defaultHandler
		}
		dtCfg := &adapters.DingTalkConfig{
			AppKey:       userCfg.IM.DingTalk.AppKey,
			AppSecret:    userCfg.IM.DingTalk.AppSecret,
			BotCode:      userCfg.IM.DingTalk.BotCode,
			CallbackURL:  userCfg.IM.DingTalk.CallbackURL,
			EncryptKey:   userCfg.IM.DingTalk.EncryptKey,
			UseWebhook:   userCfg.IM.DingTalk.UseWebhook,
			CallbackPath: "/dingtalk/webhook",
			Port:         "8081",
		}
		hosts.DingTalk = adapters.NewDingTalkAdapter(nil, dtCfg, commCfg)
		return hosts, hosts.DingTalk

	default:
		slog.Warn("unknown im.platform.provider; IM disabled", "provider", provider)
		return nil, defaultHandler
	}
}

// StartIM connects adapters to the gateway and starts listeners.
func StartIM(ctx context.Context, gw *gateway.CommunicationGateway, hosts *IMHosts) error {
	if hosts == nil {
		return nil
	}
	if hosts.Feishu != nil {
		hosts.Feishu.SetGateway(gw)
		if err := hosts.Feishu.Start(ctx); err != nil {
			return err
		}
		slog.Info("im adapter started", "provider", "feishu")
	}
	if hosts.DingTalk != nil {
		hosts.DingTalk.SetGateway(gw)
		if err := hosts.DingTalk.Start(ctx); err != nil {
			return err
		}
		slog.Info("im adapter started", "provider", "dingtalk")
	}
	return nil
}

func stringsTrim(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}
