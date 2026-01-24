package main

import (
	"context"
	"fmt"

	"github.com/mcoder2014/go_utils/log"
	"github.com/mcoder2014/go_utils/notify/feishu/custom_bot"
	"github.com/mcoder2014/go_utils/notify/gotify"
	"github.com/mcoder2014/go_utils/notify/weixin/qyapi"
)

func (r *Retry) reportErr(ctx context.Context, content string, args ...any) {
	if len(r.opt.FeishuCustomRobotURL) > 0 {
		if err := reportFeishuRobot(ctx, r.opt.FeishuCustomRobotURL, content, args...); err != nil {
			log.Ctx(ctx).WithError(err).Errorf("report feishu robot failed, content: %s", content)
		}
	}

	if len(r.opt.WechatRobotURL) > 0 {
		if err := reportWechatRobot(ctx, r.opt.WechatRobotURL, content, args...); err != nil {
			log.Ctx(ctx).WithError(err).Errorf("report wechat robot failed, content: %s", content)
		}
	}

	if r.opt.GotifyConfig != nil && r.opt.GotifyConfig.ServerURL != "" && r.opt.GotifyConfig.Token != "" {
		if err := reportGotify(ctx, r.opt.GotifyConfig, "retry_exec report fail", content, args...); err != nil {
			log.Ctx(ctx).WithError(err).Errorf("report gotify failed, content: %s", content)
		}
	}

}

func reportFeishuRobot(ctx context.Context, url string, content string, args ...any) error {
	return custom_bot.SendErrorMessage(ctx, url, "retry_exec report failed", fmt.Sprintf(content, args...))
}

func reportWechatRobot(ctx context.Context, url string, content string, args ...any) error {
	return qyapi.SendTextMessage(ctx, url, fmt.Sprintf(content, args...))
}

func reportGotify(ctx context.Context, config *GotifyConfig, title string, content string, args ...any) error {
	return gotify.SendNotification(ctx, &gotify.GotifyMessage{

		Title:    fmt.Sprintf(title),
		Message:  fmt.Sprintf(content, args...),
		Priority: 1,
		ServerOption: &gotify.ServerOption{
			ServerURL: config.ServerURL,
			Token:     config.Token,
		},
	})
}
