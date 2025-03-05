// push_test.go
package push

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

var sendMessageURL = "https://wxpusher.zjiecode.com/api/send/message"

func TestSendSuccess(t *testing.T) {
	// 创建一个模拟的 HTTP 测试服务器，模拟 WxPusher API 接口
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 可选：验证请求体内容
		var reqMsg Message
		if err := json.NewDecoder(r.Body).Decode(&reqMsg); err != nil {
			t.Errorf("解码请求体失败: %v", err)
		}
		// 返回一个模拟成功的响应
		respData := Response{
			Code:    200,
			Msg:     "OK",
			Success: true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(respData)
	}))
	defer ts.Close()

	// 在测试中将接口 URL 覆盖为测试服务器的地址
	originalURL := sendMessageURL
	sendMessageURL = ts.URL
	defer func() { sendMessageURL = originalURL }()

	// 设置环境变量（也可以直接传入 appToken 和 uid）
	appToken := "AT_fg9ETrNBSf0UwqSWTJMU6nCUyIKzrEz0"
	uid := "UID_FsE6vt9mYWsGi16fASvRC9GZCaCT"
	os.Setenv("WXPUSHER_APP_TOKEN", appToken)
	os.Setenv("WXPUSHER_UID", uid)

	// 测试发送消息
	content := "测试消息内容"
	resp, err := Send(content, "", "") // 此处传空字符串，以便从环境变量中获取
	if err != nil {
		t.Fatalf("Send 返回错误: %v", err)
	}

	if !resp.Success {
		t.Errorf("预期 success 为 true，实际为 false")
	}
	if resp.Code != 200 {
		t.Errorf("预期 Code 为 200，实际为 %d", resp.Code)
	}
	if resp.Msg != "OK" {
		t.Errorf("预期 Msg 为 OK，实际为 %s", resp.Msg)
	}
}
