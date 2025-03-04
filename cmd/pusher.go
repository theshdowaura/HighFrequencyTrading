package cmd

import (
	"HighFrequencyTrading/push"
	"github.com/spf13/cobra"
)

var (
	// 子命令的配置参数
	content  string
	appToken string
	uid      string
	// 定义子命令
	subCmd = &cobra.Command{
		Use:   "wxpusher",
		Short: "推送消息",
		Long:  "推送消息到wxpusher中",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 子命令的执行逻辑
			if content == "" || appToken == "" || uid == "" {
				return nil
			} else {
				push.Send(content, appToken, uid)
			}
			return nil
		},
	}
)
