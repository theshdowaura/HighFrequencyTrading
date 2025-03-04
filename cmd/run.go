package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"HighFrequencyTrading/auth"
	"HighFrequencyTrading/config"
	"HighFrequencyTrading/exchange"
)

// MainLogic : 原脚本的核心流程
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

	// 若 cfg.H 不为空，则强制写入
	if cfg.H != nil {
		log.Printf("[CMD] 强制设定h=%d", *cfg.H)
	}

	client := &http.Client{}

	// 2. 并发处理每个账号
	var wg sync.WaitGroup
	for _, a := range accs {
		wg.Add(1)
		// 在 cmd/run.go 的匿名函数中对每个账号解析时进行如下修改：
		go func(accountStr string) {
			defer wg.Done()
			fields := strings.Split(accountStr, "#")
			// 如果只有两个字段，默认 uid 为 phone
			if len(fields) < 2 {
				log.Printf("[Error] 账号格式错误: %s", accountStr)
				return
			}
			phone := fields[0]
			password := fields[1]
			var uid string
			if len(fields) >= 3 {
				uid = fields[len(fields)-1]
			} else {
				uid = phone
			}

			// 优先用缓存
			var ticket string
			if v, ok := g.Cache[phone]; ok {
				log.Printf("[Cache] phone=%s 使用缓存", phone)
				vm := v.(map[string]interface{})
				userId := vm["userId"].(string)
				token := vm["token"].(string)
				if tk, err := auth.GetTicket(g, phone, userId, token, client); err == nil {
					ticket = tk
				}
			}
			if ticket == "" {
				log.Printf("[PwdLogin] phone=%s", phone)
				tk, err := auth.UserLoginNormal(g, phone, password, client)
				if err != nil {
					log.Printf("[Error] phone=%s 登录失败: %v", phone, err)
					return
				}
				ticket = tk
			}
			if ticket != "" {
				Ks(g, phone, ticket, uid, client, cfg)
			}
		}(a)

	}
	wg.Wait()

	// 3. 等待到设定时间
	for {
		now := float64(time.Now().Unix())
		if now >= g.Wt {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// 4. 日志转换
	handleExchangeLog2(g)
	// 5. 检查推送
	exchange.CheckPushTime()

	log.Println("===== 电信金豆换话费(Go版) 结束 =====")
}

// Ks 对应原脚本 ks
func Ks(g *config.GlobalVars, phone, ticket, uid string, client *http.Client, cfg *config.Config) {
	log.Printf("[Ks] phone=%s ticket=%s", phone, ticket)
	// 模拟查询金豆余额
	time.Sleep(300 * time.Millisecond)

	// 假设获取到商品列表
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
	// 填入 g.Jp
	for _, it := range mockItems {
		if exchange.InStringArray(it.Title, g.MorningExchanges) {
			g.Jp["9"][it.Title] = it.ID
		} else if exchange.InStringArray(it.Title, g.AfternoonExchanges) {
			g.Jp["13"][it.Title] = it.ID
		}
	}

	// 决定上午/下午场
	nowH := time.Now().Hour()
	if nowH < 11 {
		nowH = 9
	} else {
		nowH = 13
	}
	if cfg.H != nil {
		nowH = *cfg.H
	}

	// 计算目标时间
	target := exchange.CalcT(nowH)
	g.Wt = float64(target) + g.Kswt

	// 分发 dh
	d := g.Jp[fmt.Sprintf("%d", nowH)]
	for di, aid := range d {
		if _, ok := g.Dhjl[g.Yf][di]; !ok {
			g.Dhjl[g.Yf][di] = ""
		}
		// 检查是否已兑换
		if strings.Contains(g.Dhjl[g.Yf][di], phone) {
			log.Printf("[Ks] %s %s 已兑换", phone, di)
			continue
		}
		log.Printf("[Ks] phone=%s item=%s", phone, di)

		// 如果等待超过30分钟则提前退出
		if g.Wt-float64(time.Now().Unix()) > 1800 {
			log.Println("[Ks] 等待时间超过30分钟,提前退出")
			return
		}
		go exchange.Dh(g, phone, di, aid, g.Wt, uid, client)
	}
}

// handleExchangeLog2 : 转换日志到 电信金豆换话费2.log
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
			listP := strings.Split(phones, "#")
			for _, p := range listP {
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
	data, _ := json.MarshalIndent(dhjl2, "", "  ")
	_ = os.WriteFile(config.ExchangeLogFile2, data, 0644)
}
