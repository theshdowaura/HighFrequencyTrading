package exchange

import (
	"HighFrequencyTrading/config"
	"HighFrequencyTrading/push"
	"HighFrequencyTrading/util"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// 这里原先有一个 dhjlMutex，现在已去除，统一使用 g.Mu

// One 发送最终兑换请求，成功后记录手机号
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
		// TODO: 此处最好解析响应体JSON，确认成功再做记录
		log.Printf("[One] %s 兑换 %s 成功", phone, title)

		// 写 Dhjl 需要加写锁
		g.Mu.Lock()
		phones := g.Dhjl[g.Yf][title]
		phones = append(phones, phone)
		g.Dhjl[g.Yf][title] = phones
		// 记完日志后保存
		g.SaveDhjl()
		g.Mu.Unlock()
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

// Dh 在指定时间 wt 到达后进行兑换请求
func Dh(g *config.GlobalVars, phone, title, aid string, wt float64, uid string, client *http.Client) {
	delay := time.Until(time.Unix(int64(wt), 0))
	if delay > 0 {
		time.Sleep(delay)
	}
	log.Printf("[Dh] phone=%s title=%s 开始兑换", phone, title)
	One(g, phone, title, aid, uid, client)
}

// sendWxPusher 发送消息
func sendWxPusher(uid, content string) {
	// 优先环境变量
	appToken := os.Getenv("WXPUSHER_APP_TOKEN")
	// 再看 util.Wxpusher
	var wxpusher util.Wxpusher
	if appToken == "" && wxpusher.AppToken == "" {
		log.Println("[sendWxPusher] WXPUSHER_APP_TOKEN 未配置")
		return
	} else {
		// 如果空，就用 wxpusher.AppToken；否则覆盖
		if wxpusher.AppToken != "" {
			appToken = wxpusher.AppToken
		}
	}

	if uid == "" && wxpusher.Uid == "" {
		uid = os.Getenv("WXPUSHER_UID")
		if uid == "" {
			log.Println("[sendWxPusher] WXPUSHER_UID 未配置")
			return
		}
	} else {
		if wxpusher.Uid != "" {
			uid = wxpusher.Uid
		}
	}

	resp, err := push.Send(content, appToken, uid)
	if err != nil {
		log.Printf("[sendWxPusher] error: %v", err)
		return
	}
	log.Printf("[sendWxPusher] uid=%s content=%s, response: %+v", uid, content, resp)
}

// PushSummary 生成兑换汇总并推送
func PushSummary(g *config.GlobalVars, uid string) {
	log.Println("[PushSummary] 开始生成汇总消息")

	// 读锁读取 Dhjl
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	var builder strings.Builder
	builder.WriteString("兑换汇总:\n")
	for title, phones := range g.Dhjl[g.Yf] {
		builder.WriteString(fmt.Sprintf("商品: %s\n", title))
		for _, phone := range phones {
			builder.WriteString(fmt.Sprintf("  手机号: %s\n", phone))
		}
	}
	msg := builder.String()
	sendWxPusher(uid, msg)
}

// InStringArray 判断字符串是否在切片中
func InStringArray(s string, arr []string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}

// CalcT 计算当日特定小时(h):00:00的时间戳
func CalcT(h int) int64 {
	now := time.Now()
	tm := time.Date(now.Year(), now.Month(), now.Day(), h, 0, 0, 0, now.Location())
	return tm.Unix()
}
