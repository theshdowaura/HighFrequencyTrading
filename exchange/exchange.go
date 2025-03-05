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

// One 发送最终兑换请求，兑换成功后记录手机号但不立即推送消息
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
		dhjlMutex.Lock()
		phones := g.Dhjl[g.Yf][title]
		phones = append(phones, phone)
		g.Dhjl[g.Yf][title] = phones
		dhjlMutex.Unlock()

		g.SaveDhjl()
	} else {
		log.Printf("[One] phone=%s status=%d", phone, resp.StatusCode)
	}
}

// DoHighFreqRequests 在目标时间前3秒内发送高频空请求
func DoHighFreqRequests(stop time.Time, phone string, client *http.Client, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}
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

// DoHighFreqRealRequests 在目标时间前1秒发送真实预热请求
func DoHighFreqRealRequests(stop time.Time, phone string, titles, aids []string, client *http.Client, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}
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

// Dh 在指定时间 wt 到达后正式进行兑换请求
func Dh(g *config.GlobalVars, phone, title, aid string, wt float64, uid string, client *http.Client) {
	delay := time.Until(time.Unix(int64(wt), 0))
	if delay > 0 {
		time.Sleep(delay)
	}
	log.Printf("[Dh] phone=%s title=%s 开始兑换", phone, title)
	One(g, phone, title, aid, uid, client)
}

// sendWxPusher 发送消息，消息内容包含 title 与 phone 信息
func sendWxPusher(uid, content string) {
	appToken := os.Getenv("WXPUSHER_APP_TOKEN")
	if appToken == "" {
		log.Println("[sendWxPusher] WXPUSHER_APP_TOKEN 未配置")
		return
	}
	// 如果 uid 为空，则尝试从环境变量中获取
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

// CheckPushTime 判断时间段，若在指定推送时间则生成汇总消息并推送出去
// 此函数需要传入全局变量 g 与 uid，以便获取兑换记录并进行消息推送
func CheckPushTime(g *config.GlobalVars, uid string) {
	now := time.Now()
	start1 := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 30, 0, now.Location())
	end1 := time.Date(now.Year(), now.Month(), now.Day(), 11, 0, 0, 0, now.Location())
	start2 := time.Date(now.Year(), now.Month(), now.Day(), 14, 0, 30, 0, now.Location())
	end2 := time.Date(now.Year(), now.Month(), now.Day(), 15, 0, 0, 0, now.Location())

	if (now.After(start1) && now.Before(end1)) || (now.After(start2) && now.Before(end2)) {
		log.Println("[CheckPushTime] 在推送时间,执行汇总推送消息")
		var builder strings.Builder
		builder.WriteString("兑换汇总:\n")
		// 假设 g.Dhjl 为 map，其中 key 为产品分类（例如 g.Yf），value 为 map[title][]phone
		for title, phones := range g.Dhjl[g.Yf] {
			builder.WriteString(fmt.Sprintf("商品: %s\n", title))
			for _, phone := range phones {
				builder.WriteString(fmt.Sprintf("  手机号: %s\n", phone))
			}
		}
		// 用汇总内容替换原来的 msg 变量
		msg := builder.String()
		sendWxPusher(uid, msg)
	} else {
		log.Println("[CheckPushTime] 不在推送时间")
	}
}

// InStringArray 判断字符串是否在数组中
func InStringArray(s string, arr []string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}

// CalcT 计算当天 hour=h, minute=00, second=00 的时间戳
func CalcT(h int) int64 {
	now := time.Now()
	tm := time.Date(now.Year(), now.Month(), now.Day(), h, 0, 0, 0, now.Location())
	return tm.Unix()
}
