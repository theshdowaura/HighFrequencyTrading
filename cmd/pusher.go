package cmd

import (
	"HighFrequencyTrading/util"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"os"
)

var (
	subCmd = &cobra.Command{
		Use:   "wxpusher",
		Short: "推送消息",
		Long:  "推送消息到wxpusher",
		Example: `  telecom wxpusher -a <apptoken> -u <uid>
  或者通过配置文件 wxpusher.yaml 设置`,
		RunE: func(cmd *cobra.Command, args []string) error {

			var wxpusher util.Wxpusher
			if wxpusher.AppToken != "" && wxpusher.Uid != "" {
				return nil // 使用命令行参数
			}

			// 读取配置文件
			data, err := os.ReadFile("wxpusher.yaml")
			if err != nil {
				return err
			}

			// 解析YAML
			if err := yaml.Unmarshal(data, &wxpusher); err != nil {
				return err
			}

			// 验证配置
			if wxpusher.AppToken == "" || wxpusher.Uid == "" {
				println("未设置推送详细配置")
				return nil
			}

			// 这里添加您的业务逻辑
			return nil
		},
	}
)
