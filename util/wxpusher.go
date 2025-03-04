package util

// Wxpusher 微信推送配置结构体
type Wxpusher struct {
	AppToken string `yaml:"appToken" json:"appToken"`
	Uid      string `yaml:"uid" json:"uid"`
}
