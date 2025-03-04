package cmd

import (
	"HighFrequencyTrading/util"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"os"
)

// WxpusherConfig YAML配置文件对应的结构体
type WxpusherConfig struct {
	AppToken string `yaml:"app_token"`
	Uid      string `yaml:"uid"`
	// 如果有其他配置项，可以继续添加
	Topic string `yaml:"topic,omitempty"`
	Debug bool   `yaml:"debug,omitempty"`
}

// LoadConfig 加载YAML配置文件
func LoadConfig(filename string) (*WxpusherConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	config := &WxpusherConfig{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

var (
	AppToken string
	Uid      string

	subCmd = &cobra.Command{
		Use:   "wxpusher",
		Short: "推送消息",
		Long:  "推送消息到wxpusher",
		Example: `  telecom wxpusher -a <AppToken> -u <Uid>
  或者通过配置文件 wxpusher.yaml 设置`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var wxpusher util.Wxpusher

			// 优先从环境变量获取配置
			AppTokenEnv := os.Getenv("WXPUSHER_APP_TOKEN")
			UidEnv := os.Getenv("WXPUSHER_UID")

			// 如果环境变量存在，则使用环境变量的值
			if AppTokenEnv != "" && UidEnv != "" {
				wxpusher.AppToken = AppTokenEnv
				wxpusher.Uid = UidEnv
			} else if AppToken != "" && Uid != "" {
				// 如果命令行参数存在，使用命令行参数
				wxpusher.AppToken = AppToken
				wxpusher.Uid = Uid
			} else {
				// 环境变量和命令行参数都为空时，才尝试读取配置文件
				config, err := LoadConfig("wxpusher.yaml")
				if err != nil {
					return err
				}

				// 将配置应用到wxpusher实例
				wxpusher.AppToken = config.AppToken
				wxpusher.Uid = config.Uid
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
