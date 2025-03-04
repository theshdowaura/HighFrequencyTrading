package exchange

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"HighFrequencyTrading/config"
)

// ExchangeOne 最终发送兑换请求
func ExchangeOne(g *config.GlobalVars, phone, title, aid, uid string, client *http.Client) {
	url := "https://wapact.189.cn:9001/gateway/standExchange/detailNew/exchange"
	body := fmt.Sprintf(`{"activityId":"%s"}`, aid)
	// 发起请求
	resp, err := client.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		log.Printf("[ExchangeOne] err=%v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		// 假设成功
		log.Printf("[ExchangeOne] %s 兑换 %s 成功", phone, title)
		// 更新日志
		g.Dhjl[g.Yf][title] += "#" + phone
		g.SaveDhjl()
		// 推送消息
		sendWxPusher(uid, fmt.Sprintf("%s:%s 兑换成功", phone, title))
	} else {
		log.Printf("[ExchangeOne] status=%d", resp.StatusCode)
	}
}

// DoHighFreqRequests 强发
func DoHighFreqRequests(stop time.Time, phone string, client *http.Client, wg *sync.WaitGroup) {
	defer wg.Done()
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			if time.Now().After(stop) {
				log.Printf("[DoHighFreqRequests] phone=%s done", phone)
				return
			}
			go func() {
				resp, err := client.Get("https://example.com/ping")
				if err == nil {
					resp.Body.Close()
				}
			}()
		}
	}
}

// DoHighFreqRealRequests 真实预热
func DoHighFreqRealRequests(stop time.Time, phone, ticket string, titles, aids []string, client *http.Client, wg *sync.WaitGroup) {
	defer wg.Done()
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			if time.Now().After(stop) {
				log.Printf("[DoHighFreqRealRequests] phone=%s done", phone)
				return
			}
			if len(titles) == 0 {
				continue
			}
			i := time.Now().UnixNano() % int64(len(titles))
			// title := titles[i]
			aid := aids[i]
			go func() {
				body := fmt.Sprintf(`{"activityId":"%s","warmupFlag":true}`, aid)
				client.Post("https://wapact.189.cn:9001/gateway/standExchange/detailNew/exchange",
					"application/json", strings.NewReader(body))
			}()
		}
	}
}

// Dh 函数，对应原脚本的 dh
func Dh(g *config.GlobalVars, phone, title, aid string, wt float64, uid string, client *http.Client) {
	for {
		if float64(time.Now().Unix()) >= wt {
			break
		}
		time.Sleep(time.Second)
	}
	log.Printf("[Dh] phone=%s title=%s 开始兑换", phone, title)
	ExchangeOne(g, phone, title, aid, uid, client)
}

// sendWxPusher 发送消息
func sendWxPusher(uid, content string) {
	// 可根据实际情况, 调用真正接口
	log.Printf("[sendWxPusher] uid=%s content=%s", uid, content)
}

// CheckPushTime 调用推送脚本
func CheckPushTime() {
	now := time.Now()
	start1 := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 30, 0, now.Location())
	end1 := time.Date(now.Year(), now.Month(), now.Day(), 11, 0, 0, 0, now.Location())
	start2 := time.Date(now.Year(), now.Month(), now.Day(), 14, 0, 30, 0, now.Location())
	end2 := time.Date(now.Year(), now.Month(), now.Day(), 15, 0, 0, 0, now.Location())

	if (now.After(start1) && now.Before(end1)) || (now.After(start2) && now.Before(end2)) {
		log.Println("[CheckPushTime] 在推送时间, 执行 python 汇总推送脚本")
		cmd := exec.Command("python", "汇总推送.py")
		cmd.Run()
	} else {
		log.Println("[CheckPushTime] 不在推送时间")
	}
}

// InStringArray 辅助判断
func InStringArray(s string, arr []string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}

// CalcT(h) 计算当天 hour=h, minute=59, second=20
func CalcT(h int) int64 {
	now := time.Now()
	tm := time.Date(now.Year(), now.Month(), now.Day(), h, 59, 20, 0, now.Location())
	return tm.Unix()
}
