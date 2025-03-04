package push

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
)

// Message 定义发送消息的请求结构
type Message struct {
	AppToken    string   `json:"appToken"`
	Content     string   `json:"content"`
	ContentType int      `json:"contentType"`
	UIDs        []string `json:"uids"`
}

// Response 定义API响应结构
type Response struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Success bool   `json:"success"`
}

// Send 发送消息到WxPusher
func Send(content, appToken, uid string) (*Response, error) {
	if appToken == "" || uid == "" {
		return nil, errors.New("WXPUSHER_APP_TOKEN 或 WXPUSHER_UID 未设置,将自动使用环境变量")
	} else {
		appToken = os.Getenv("WXPUSHER_APP_TOKEN")
		uid = os.Getenv("WXPUSHER_UID")
		print(appToken, uid)
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
