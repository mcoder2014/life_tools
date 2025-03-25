package main

import (
	"context"
	"fmt"

	"github.com/mcoder2014/go_utils/log"
	"github.com/mcoder2014/go_utils/notify/feishu/custom_bot"
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
}

func reportFeishuRobot(ctx context.Context, url string, content string, args ...any) error {
	return custom_bot.SendErrorMessage(ctx, url, "retry_exec report failed", fmt.Sprintf(content, args...))
}

func reportWechatRobot(ctx context.Context, url string, content string, args ...any) error {
	return qyapi.SendTextMessage(ctx, url, fmt.Sprintf(content, args...))
}
