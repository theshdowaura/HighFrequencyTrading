package cmd

import (
	"HighFrequencyTrading/util"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"os"
)

// WxpusherConfig YAML配置文件对应的结构体
type WxpusherConfig struct {
	AppToken string `yaml:"appToken"`
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

	// 重命名为 wxpusherCmd，避免和全局命令冲突
	wxpusherCmd = &cobra.Command{
		Use:   "wxpusher",
		Short: "推送消息",
		Long:  "推送消息到 wxpusher",
		Example: `  telecom wxpusher -a <AppToken> -u <Uid>
  或者通过配置文件 wxpusher.yaml 设置`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var wxpusher util.Wxpusher

			// 优先从环境变量获取配置
			AppTokenEnv := os.Getenv("WXPUSHER_APP_TOKEN")
			UidEnv := os.Getenv("WXPUSHER_UID")

			if AppTokenEnv != "" && UidEnv != "" {
				wxpusher.AppToken = AppTokenEnv
				wxpusher.Uid = UidEnv
			} else if AppToken != "" && Uid != "" {
				wxpusher.AppToken = AppToken
				wxpusher.Uid = Uid
			} else {
				// 当环境变量和命令行参数均为空时，尝试读取配置文件
				config, err := LoadConfig("wxpusher.yaml")
				if err != nil {
					return err
				}
				wxpusher.AppToken = config.AppToken
				wxpusher.Uid = config.Uid
			}

			if wxpusher.AppToken == "" || wxpusher.Uid == "" {
				println("未设置推送详细配置 (appToken/uid)")
				return nil
			}

			// 在这里添加您的 wxpusher 业务逻辑（如实际发送推送）...

			return nil
		},
	}
)

func init() {
	// 使用 StringVarP 定义带有短标志的参数
	wxpusherCmd.Flags().StringVarP(&AppToken, "app-token", "a", "", "wxpusher的apptoken")
	wxpusherCmd.Flags().StringVarP(&Uid, "uid", "u", "", "wxpusher的uid的值")
}
