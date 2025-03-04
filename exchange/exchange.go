package exchange

import (
	"HighFrequencyTrading/config"
	"HighFrequencyTrading/push"
	"HighFrequencyTrading/util"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

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
		g.Dhjl[g.Yf][title] += "#" + phone
		g.SaveDhjl()
		sendWxPusher(uid, fmt.Sprintf("%s:%s 兑换成功", phone, title))
	} else {
		log.Printf("[One] phone=%s status=%d", phone, resp.StatusCode)
	}
}

// DoHighFreqRequests : 目标时间前3秒空请求强发
func DoHighFreqRequests(stop time.Time, phone string, client *http.Client, wg *sync.WaitGroup) {
	defer wg.Done()
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			now := time.Now()
			if now.After(stop) {
				log.Printf("[DoHighFreqRequests] phone=%s done", phone)
				return
			}
			go func() {
				resp, err := client.Get("https://wapact.189.cn:9001/gateway/standExchange/detailNew/exchange")
				log.Printf("[DoHighFreqRealRequests] phone=%s title=%s(强发)", phone)
				if err == nil {
					resp.Body.Close()
				}
			}()
		}
	}
}

// DoHighFreqRealRequests : 目标时间前1秒“真实预热”
func DoHighFreqRealRequests(stop time.Time, phone string, titles, aids []string, client *http.Client, wg *sync.WaitGroup) {
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
			title := titles[i]
			aid := aids[i]
			go func() {
				body := fmt.Sprintf(`{"activityId":"%s","warmupFlag":true}`, aid)
				client.Post("https://wapact.189.cn:9001/gateway/standExchange/detailNew/exchange", "application/json", strings.NewReader(body))
				log.Printf("[DoHighFreqRealRequests] phone=%s title=%s(预热)", phone, title)
			}()
		}
	}
}

// Dh : 在 wt 时间到达后正式进行兑换
func Dh(g *config.GlobalVars, phone, title, aid string, wt float64, uid string, client *http.Client) {
	for {
		if float64(time.Now().Unix()) >= wt {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	log.Printf("[Dh] phone=%s title=%s 开始兑换", phone, title)
	One(g, phone, title, aid, uid, client)
}

// sendWxPusher : 发送消息
func sendWxPusher(uid, content string) {
	// 可根据实际情况,调用真实接口
	var wxpusher util.Wxpusher
	push.Send(content, wxpusher.AppToken, uid)
	log.Printf("[sendWxPusher] uid=%s content=%s", uid, content)
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
		var Msg push.Message
		var Util util.Wxpusher
		sendWxPusher(Util.Uid, Msg.Content)
	} else {
		log.Println("[CheckPushTime] 不在推送时间")
	}
}

// InStringArray : 判断字符串是否在数组
func InStringArray(s string, arr []string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}

// CalcT : 计算当天 hour=h, minute=59, second=20
func CalcT(h int) int64 {
	now := time.Now()
	tm := time.Date(now.Year(), now.Month(), now.Day(), h, 59, 20, 0, now.Location())
	return tm.Unix()
}
