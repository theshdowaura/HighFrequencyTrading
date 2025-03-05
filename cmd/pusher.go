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
	Topic    string `yaml:"topic,omitempty"`
	Debug    bool   `yaml:"debug,omitempty"`
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

	// CLI子命令优先级顺序:
	// 1. 环境变量 (最高)
	// 2. CLI 参数
	// 3. YAML 配置文件 (最低)
	wxpusherCmd = &cobra.Command{
		Use:   "wxpusher",
		Short: "推送消息",
		Long:  "推送消息到 wxpusher",
		Example: `telecom wxpusher -a <AppToken> -u <Uid>
或通过配置文件 wxpusher.yaml 设置`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var wxpusher util.Wxpusher

			// 按优先级获取配置：环境变量 → CLI参数 → YAML配置文件
			if envAppToken, envUid := os.Getenv("WXPUSHER_APP_TOKEN"), os.Getenv("WXPUSHER_UID"); envAppToken != "" && envUid != "" {
				// 优先级 1：环境变量
				wxpusher.AppToken = envAppToken
				wxpusher.Uid = envUid
			} else if AppToken != "" && Uid != "" {
				// 优先级 2：CLI 参数
				wxpusher.AppToken = AppToken
				wxpusher.Uid = Uid
			} else {
				// 优先级 3：YAML 配置文件
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

			// 这里可以添加实际的推送业务逻辑
			// 例如调用推送接口 sendWxPusher(wxpusher.AppToken, wxpusher.Uid, msg)

			return nil
		},
	}
)

func init() {
	// 定义 CLI 参数
	wxpusherCmd.Flags().StringVarP(&AppToken, "app-token", "a", "", "wxpusher的appToken")
	wxpusherCmd.Flags().StringVarP(&Uid, "uid", "u", "", "wxpusher的uid")
}
