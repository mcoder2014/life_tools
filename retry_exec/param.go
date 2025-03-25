package main

type ExecOption struct {
	// 重试次数
	RetryTimes int `json:"retry_times"`

	// 重试间隔，单位毫秒
	RetryInterval int `json:"retry_interval"`

	// 最大重试间隔，单位毫秒
	MaxRetryInterval int `json:"max_retry_interval"`

	// 飞书自定义机器人回调地址
	FeishuCustomRobotURL string `json:"feishu_custom_robot_url"`

	// 微信自定义机器人回调地址
	WechatRobotURL string `json:"wechat_robot_url"`

	// 日志目录
	Log string `json:"log"`
}
