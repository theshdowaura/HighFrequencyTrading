package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"HighFrequencyTrading/auth"
	"HighFrequencyTrading/config"
	"HighFrequencyTrading/exchange"
	"net/http"
)

// MainLogic 实现原脚本的主流程
func MainLogic(cfg *config.Config) {
	log.Println("===== 电信金豆换话费(Go版) 启动 =====")

	// 1. 初始化全局变量
	g := config.InitGlobalVars(cfg)

	if cfg.Jdhf == "" {
		log.Println("[Error] 未检测到账号信息 jdhf, 退出。")
		return
	}
	accs := strings.Split(cfg.Jdhf, "&")
	log.Printf("检测到 %d 个账号", len(accs))

	// 2. 如果 cfg.H 不为空, 写入 g
	if cfg.H != nil {
		hVal := *cfg.H
		log.Printf("[CMD] 强制设定h=%d", hVal)
	}

	// 创建一个 http.Client
	client := &http.Client{}

	// 3. 并发处理每个账号
	var wg sync.WaitGroup
	for _, a := range accs {
		wg.Add(1)
		go func(accountStr string) {
			defer wg.Done()
			fields := strings.Split(accountStr, "#")
			if len(fields) < 3 {
				log.Printf("[Error] 账号格式有误: %s", accountStr)
				return
			}
			phone := fields[0]
			password := fields[1]
			uid := fields[len(fields)-1]

			// 尝试缓存
			var ticket string
			if v, ok := g.Cache[phone]; ok {
				log.Printf("[Cache] phone=%s 使用缓存", phone)
				mm := v.(map[string]interface{})
				userId := mm["userId"].(string)
				token := mm["token"].(string)
				if tk, err := auth.GetTicket(g, phone, userId, token, client); err == nil {
					ticket = tk
				}
			}
			if ticket == "" {
				log.Printf("[PwdLogin] phone=%s", phone)
				tk, err := auth.UserLoginNormal(g, phone, password, client)
				if err == nil {
					ticket = tk
				} else {
					log.Printf("[Error] phone=%s 登录失败: %v", phone, err)
					return
				}
			}
			if ticket != "" {
				Ks(g, phone, ticket, uid, client, cfg)
			}
		}(a)
	}
	wg.Wait()

	// 等待到设定时间
	for {
		now := float64(time.Now().Unix())
		if now >= g.Wt {
			break
		}
		time.Sleep(time.Second)
	}

	// 日志转换
	handleExchangeLog2(g)
	exchange.CheckPushTime()

	log.Println("===== 电信金豆换话费(Go版) 结束 =====")
}

// Ks 对应原脚本 ks
func Ks(g *config.GlobalVars, phone, ticket, uid string, client *http.Client, cfg *config.Config) {
	log.Printf("[Ks] phone=%s ticket=%s", phone, ticket)
	// 模拟查询金豆余额...
	time.Sleep(300 * time.Millisecond)

	// 假设获取商品列表...
	mockItems := []struct {
		Title string
		ID    string
	}{
		{"0.5元话费", "aid_0.5"},
		{"5元话费", "aid_5"},
		{"6元话费", "aid_6"},
		{"1元话费", "aid_1"},
		{"10元话费", "aid_10"},
		{"3元话费", "aid_3"},
	}
	for _, it := range mockItems {
		if exchange.InStringArray(it.Title, g.MorningExchanges) {
			g.Jp["9"][it.Title] = it.ID
		} else if exchange.InStringArray(it.Title, g.AfternoonExchanges) {
			g.Jp["13"][it.Title] = it.ID
		}
	}

	nowH := time.Now().Hour()
	if nowH < 11 {
		nowH = 9
	} else {
		nowH = 13
	}
	if cfg.H != nil {
		nowH = *cfg.H
	}

	// 计算 wt
	target := exchange.CalcT(nowH)
	g.Wt = float64(target) + g.Kswt

	// 分发到 Dh
	d := g.Jp[fmt.Sprintf("%d", nowH)]
	for di, aid := range d {
		if _, ok := g.Dhjl[g.Yf][di]; !ok {
			g.Dhjl[g.Yf][di] = ""
		}
		if strings.Contains(g.Dhjl[g.Yf][di], phone) {
			log.Printf("[Ks] %s %s 已兑换", phone, di)
			continue
		}
		log.Printf("[Ks] phone=%s item=%s", phone, di)
		if g.Wt-float64(time.Now().Unix()) > 1800 {
			log.Println("等待时间超过30分钟，提前退出")
			return
		}
		go exchange.Dh(g, phone, di, aid, g.Wt, uid, client)
	}
}

// handleExchangeLog2 转换日志
func handleExchangeLog2(g *config.GlobalVars) {
	nowMonth := time.Now().Format("200601")
	oldLog := g.Dhjl
	dhjl2 := make(map[string]map[string][]string)

	if oldLog[nowMonth] != nil {
		for fee, phones := range oldLog[nowMonth] {
			phones = strings.Trim(phones, "#")
			if phones == "" {
				continue
			}
			splitP := strings.Split(phones, "#")
			for _, p := range splitP {
				if p == "" {
					continue
				}
				if _, ok := dhjl2[p]; !ok {
					dhjl2[p] = make(map[string][]string)
				}
				if _, ok := dhjl2[p][nowMonth]; !ok {
					dhjl2[p][nowMonth] = []string{}
				}
				dhjl2[p][nowMonth] = append(dhjl2[p][nowMonth], fee)
			}
		}
	}
	// 写入 电信金豆换话费2.log
	data, _ := configJsonMarshalIndent(dhjl2)
	_ = WriteFileSafe(config.ExchangeLogFile2, data)
}

func configJsonMarshalIndent(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

func WriteFileSafe(file string, data []byte) error {
	return os.WriteFile(file, data, 0644)
}
