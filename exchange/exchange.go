package exchange

import (
	"HighFrequencyTrading/config"
	"HighFrequencyTrading/push"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// dhjlMutex 用于保护 g.Dhjl 的并发读写
var dhjlMutex sync.Mutex

// One : 发送最终兑换请求
func One(g *config.GlobalVars, phone, title, aid, uid string, client *http.Client) {
	url := "https://wapact.189.cn:9001/gateway/standExchange/detailNew/exchange"
	body := fmt.Sprintf(`{"activityId":"%s"}`, aid)
	resp, err := client.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		log.Printf("[One] err=%v phone=%s", err, phone)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		log.Printf("[One] %s 兑换 %s 成功", phone, title)
		// 更新兑换日志时加锁保护
		dhjlMutex.Lock()
		g.Dhjl[g.Yf][title] += "#" + phone
		dhjlMutex.Unlock()

		g.SaveDhjl()
		sendWxPusher(uid, fmt.Sprintf("%s:%s 兑换成功", phone, title))
	} else {
		log.Printf("[One] phone=%s status=%d", phone, resp.StatusCode)
	}
}

// DoHighFreqRequests : 目标时间前3秒空请求强发
func DoHighFreqRequests(stop time.Time, phone string, client *http.Client, wg *sync.WaitGroup) {
	defer func() {
		if wg != nil {
			wg.Done()
		}
	}()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if time.Now().After(stop) {
				log.Printf("[DoHighFreqRequests] phone=%s done", phone)
				return
			}
			go func() {
				resp, err := client.Get("https://wapact.189.cn:9001/gateway/standExchange/detailNew/exchange")
				if err != nil {
					log.Printf("[DoHighFreqRequests] phone=%s error: %v", phone, err)
					return
				}
				resp.Body.Close()
				log.Printf("[DoHighFreqRequests] phone=%s 强发请求发送", phone)
			}()
		}
	}
}

// DoHighFreqRealRequests : 目标时间前1秒“真实预热”
func DoHighFreqRealRequests(stop time.Time, phone string, titles, aids []string, client *http.Client, wg *sync.WaitGroup) {
	defer func() {
		if wg != nil {
			wg.Done()
		}
	}()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if time.Now().After(stop) {
				log.Printf("[DoHighFreqRealRequests] phone=%s done", phone)
				return
			}
			if len(titles) == 0 {
				continue
			}
			i := time.Now().UnixNano() % int64(len(titles))
			title := titles[i]
			aid := aids[i]
			go func(title, aid string) {
				body := fmt.Sprintf(`{"activityId":"%s","warmupFlag":true}`, aid)
				resp, err := client.Post("https://wapact.189.cn:9001/gateway/standExchange/detailNew/exchange", "application/json", strings.NewReader(body))
				if err != nil {
					log.Printf("[DoHighFreqRealRequests] phone=%s error: %v", phone, err)
					return
				}
				resp.Body.Close()
				log.Printf("[DoHighFreqRealRequests] phone=%s title=%s(预热) 请求发送", phone, title)
			}(title, aid)
		}
	}
}

// Dh : 在 wt 时间到达后正式进行兑换
func Dh(g *config.GlobalVars, phone, title, aid string, wt float64, uid string, client *http.Client) {
	// 计算等待时间，直接 sleep 整体延时
	delay := time.Until(time.Unix(int64(wt), 0))
	if delay > 0 {
		time.Sleep(delay)
	}
	log.Printf("[Dh] phone=%s title=%s 开始兑换", phone, title)
	One(g, phone, title, aid, uid, client)
}

// sendWxPusher : 发送消息（从环境变量中获取推送配置）
func sendWxPusher(uid, content string) {
	appToken := os.Getenv("WXPUSHER_APP_TOKEN")
	if appToken == "" {
		log.Println("[sendWxPusher] WXPUSHER_APP_TOKEN 未配置")
		return
	}
	// 如果 uid 为空，则尝试获取环境变量
	if uid == "" {
		uid = os.Getenv("WXPUSHER_UID")
		if uid == "" {
			log.Println("[sendWxPusher] WXPUSHER_UID 未配置")
			return
		}
	}
	resp, err := push.Send(content, appToken, uid)
	if err != nil {
		log.Printf("[sendWxPusher] error: %v", err)
		return
	}
	log.Printf("[sendWxPusher] uid=%s content=%s, response: %+v", uid, content, resp)
}

// CheckPushTime : 判断时间段后执行 python 汇总推送
func CheckPushTime() {
	now := time.Now()
	start1 := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 30, 0, now.Location())
	end1 := time.Date(now.Year(), now.Month(), now.Day(), 11, 0, 0, 0, now.Location())
	start2 := time.Date(now.Year(), now.Month(), now.Day(), 14, 0, 30, 0, now.Location())
	end2 := time.Date(now.Year(), now.Month(), now.Day(), 15, 0, 0, 0, now.Location())

	if (now.After(start1) && now.Before(end1)) || (now.After(start2) && now.Before(end2)) {
		log.Println("[CheckPushTime] 在推送时间,执行推送消息")
		msg := push.Message{
			Content: "汇总推送消息", // 根据实际情况设置
		}
		uid := os.Getenv("WXPUSHER_UID")
		if uid == "" {
			log.Println("[CheckPushTime] WXPUSHER_UID 未配置")
			return
		}
		sendWxPusher(uid, msg.Content)
	} else {
		log.Println("[CheckPushTime] 不在推送时间")
	}
}

// InStringArray : 判断字符串是否在数组中
func InStringArray(s string, arr []string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}

// CalcT : 计算当天 hour=h, minute=59, second=59
func CalcT(h int) int64 {
	now := time.Now()
	// 修正：将 nanosecond 参数设为 0
	tm := time.Date(now.Year(), now.Month(), now.Day(), h, 59, 59, 0, now.Location())
	return tm.Unix()
}
