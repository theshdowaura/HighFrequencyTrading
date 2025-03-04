package cmd

import (
	"HighFrequencyTrading/util"
	"github.com/spf13/cobra"
	"os"
)

var (
	// 子命令的配置参数
	content string

	// 定义子命令
	subCmd = &cobra.Command{
		Use:   "wxpusher",
		Short: "推送消息",
		Long:  "推送消息到wxpusher中",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 子命令的执行逻辑
			var wxpusher util.Wxpusher
			if wxpusher.AppToken == "" || wxpusher.Uid == "" {
				println("未设置推送详细配置")
				return nil
			} else {
				os.ReadFile("wxpusher.yaml")
			}
			return nil
		},
	}
)
