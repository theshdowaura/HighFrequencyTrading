// exchange_test.go
package exchange

import (
	"HighFrequencyTrading/config"
	"os"
	"testing"
	"time"
)

// 定义默认的推送时间段（当天的 18:00:30 到 19:00:00）
var (
	pushStart = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 18, 0, 30, 0, time.Local)
	pushEnd   = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 19, 0, 0, 0, time.Local)
)

func TestCheckPushTime(t *testing.T) {
	// 设置测试环境下的环境变量，确保 sendWxPusher 可被调用
	os.Setenv("WXPUSHER_APP_TOKEN", "AT_fg9ETrNBSf0UwqSWTJMU6nCUyIKzrEz0")
	os.Setenv("WXPUSHER_UID", "UID_FsE6vt9mYWsGi16fASvRC9GZCaCT")

	// 构造一个伪造的全局变量实例
	g := &config.GlobalVars{
		Yf: "testCategory",
		Dhjl: map[string]map[string][]string{
			"testCategory": {
				"商品A": {"13800138000", "13900139000"},
				"商品B": {"13700137000"},
			},
		},
	}

	now := time.Now()
	// 将测试推送时间设为当前时间前后 1 分钟，即确保当前时间处于此范围内
	start := now.Add(-1 * time.Minute)
	end := now.Add(1 * time.Minute)

	// 在测试中覆盖全局变量 pushStart 和 pushEnd
	pushStart = start
	pushEnd = end

	originalStart := time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 30, 0, now.Location())
	originalEnd := time.Date(now.Year(), now.Month(), now.Day(), 19, 0, 0, 0, now.Location())

	t.Logf("原推送时间段: %v - %v", originalStart, originalEnd)
	t.Logf("测试推送时间段设为: %v - %v", start, end)

	t.Log("执行 CheckPushTime 测试")
	PushSummary(g, "dummyUid")
}
