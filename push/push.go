package push

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
)

// Message 请求体
type Message struct {
	AppToken    string   `json:"appToken"`
	Content     string   `json:"content"`
	ContentType int      `json:"contentType"`
	UIDs        []string `json:"uids"`
}

// Response 响应体
type Response struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Success bool   `json:"success"`
}

// Send 发送消息到WxPusher
func Send(content, appToken, uid string) (*Response, error) {
	// 若未传递，则从环境变量获取
	if appToken == "" {
		appToken = os.Getenv("WXPUSHER_APP_TOKEN")
	}
	if uid == "" {
		uid = os.Getenv("WXPUSHER_UID")
	}
	if appToken == "" || uid == "" {
		return nil, errors.New("WXPUSHER_APP_TOKEN 或 WXPUSHER_UID 未设置")
	}

	message := Message{
		AppToken:    appToken,
		Content:     content,
		ContentType: 1,
		UIDs:        []string{uid},
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(
		"https://wxpusher.zjiecode.com/api/send/message",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	return &response, nil
}
