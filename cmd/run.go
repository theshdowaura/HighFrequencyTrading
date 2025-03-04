// cmd/exchange.go
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

	"HighFrequencyTrading/config"
	"HighFrequencyTrading/exchange"
	"HighFrequencyTrading/sign"
)

// MainLogic 是程序入口
func MainLogic(cfg *config.Config) {
	log.Println("===== 高频交易系统启动 =====")

	// 1. 初始化全局配置
	g := config.InitGlobalVars(cfg)
	if cfg.Jdhf == "" {
		log.Println("[Error] 未检测到账号信息，退出")
		return
	}

	accounts := strings.Split(cfg.Jdhf, "&")
	log.Printf("检测到 %d 个账号", len(accounts))

	if cfg.H != nil {
		log.Printf("[CMD] 强制设定h=%d", *cfg.H)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// 2. 并发处理每个账号
	var wg sync.WaitGroup
	for _, account := range accounts {
		wg.Add(1)
		go processAccount(account, g, client, cfg, &wg)
	}
	wg.Wait()

	// 3. 等待至目标时间
	waitUntilTargetTime(g.Wt)

	// 4. 处理日志
	handleExchangeLog(g)

	// 5. 检查推送
	exchange.CheckPushTime()

	log.Println("===== 高频交易系统结束 =====")
}

func processAccount(accountStr string, g *config.GlobalVars, client *http.Client, cfg *config.Config, wg *sync.WaitGroup) {
	defer wg.Done()

	// 解析账号信息
	fields := strings.Split(accountStr, "#")
	if len(fields) < 2 {
		log.Printf("[Error] 账号格式错误: %s", accountStr)
		return
	}
	phone := fields[0]
	password := fields[1]
	uid := getUID(fields, phone)

	// 获取 ticket（优先尝试缓存，失败则重新登录）
	ticket := getTicket(phone, password, g)
	if ticket == "" {
		return
	}

	// 执行交易逻辑
	executeTrading(g, phone, ticket, uid, client, cfg)
}

func getUID(fields []string, phone string) string {
	if len(fields) >= 3 {
		return fields[len(fields)-1]
	}
	return phone
}

func getTicket(phone, password string, g *config.GlobalVars) string {
	// 尝试使用缓存
	if v, ok := g.Cache[phone]; ok {
		log.Printf("[Cache] phone=%s 使用缓存", phone)
		vm := v.(map[string]interface{})
		userId := vm["userId"].(string)
		token := vm["token"].(string)
		if tk, err := sign.GetTicket(phone, userId, token); err == nil {
			return tk
		}
	}

	// 缓存无效，重新登录
	log.Printf("[Login] phone=%s", phone)
	tk, err := sign.UserLoginNormal(phone, password)
	if err != nil {
		log.Printf("[Error] phone=%s 登录失败: %v", phone, err)
		return ""
	}
	return tk
}

func executeTrading(g *config.GlobalVars, phone, ticket, uid string, client *http.Client, cfg *config.Config) {
	log.Printf("[Trading] phone=%s", phone)

	// 获取商品列表，并更新全局产品信息
	items := getTradeItems()
	updateGlobalProducts(g, items)

	// 确定交易时间（9点或13点）
	tradeHour := determineTradeHour(cfg)
	targetTime := calculateTargetTime(tradeHour, g)
	products := g.Jp[fmt.Sprintf("%d", tradeHour)]

	// 收集商品信息
	titles, aids := collectProductInfo(products)

	// 执行预热阶段（抢发和实际预热各自独立调度）
	executeWarmupStages(g, phone, titles, aids, client, targetTime)

	// 执行实际交易
	executeTrades(g, phone, products, uid, client)
}

func getTradeItems() []struct {
	Title string
	ID    string
} {
	return []struct {
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
}

func updateGlobalProducts(g *config.GlobalVars, items []struct{ Title, ID string }) {
	for _, item := range items {
		if exchange.InStringArray(item.Title, g.MorningExchanges) {
			g.Jp["9"][item.Title] = item.ID
		} else if exchange.InStringArray(item.Title, g.AfternoonExchanges) {
			g.Jp["13"][item.Title] = item.ID
		}
	}
}

func determineTradeHour(cfg *config.Config) int {
	if cfg.H != nil {
		return *cfg.H
	}
	nowH := time.Now().Hour()
	if nowH < 11 {
		return 9
	}
	return 13
}

func calculateTargetTime(hour int, g *config.GlobalVars) float64 {
	target := exchange.CalcT(hour)
	return float64(target) + g.Kswt
}

func collectProductInfo(products map[string]string) ([]string, []string) {
	var titles, aids []string
	for title, aid := range products {
		titles = append(titles, title)
		aids = append(aids, aid)
	}
	return titles, aids
}

// executeWarmupStages 独立调度预热阶段：3秒前抢发、1秒前实际预热
func executeWarmupStages(g *config.GlobalVars, phone string, titles, aids []string, client *http.Client, targetTime float64) {
	var wg sync.WaitGroup

	// 计算调度时间点
	baseTime := time.Unix(int64(targetTime), 0)
	emptyRequestTime := baseTime.Add(-3 * time.Second)
	realRequestTime := baseTime.Add(-1 * time.Second)

	// 阶段1：3秒前的空请求抢发
	if time.Now().Before(emptyRequestTime) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			scheduleStage(emptyRequestTime, "抢发阶段", func() {
				exchange.DoHighFreqRequests(emptyRequestTime, phone, client, nil)
			})
		}()
	} else {
		log.Println("[Warmup] 抢发阶段时间点已过，跳过")
	}

	// 阶段2：1秒前的实际预热
	if time.Now().Before(realRequestTime) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			scheduleStage(realRequestTime, "预热阶段", func() {
				exchange.DoHighFreqRealRequests(realRequestTime, phone, titles, aids, client, nil)
			})
		}()
	} else {
		log.Println("[Warmup] 预热阶段时间点已过，跳过")
	}

	wg.Wait()
}

// scheduleStage 等待指定时间点，然后执行任务
func scheduleStage(scheduledTime time.Time, stageName string, task func()) {
	waitDuration := time.Until(scheduledTime)
	if waitDuration > 0 {
		log.Printf("[Stage %s] 等待 %v 后启动，计划时间：%v", stageName, waitDuration, scheduledTime)
		time.Sleep(waitDuration)
	}
	log.Printf("[Stage %s] 启动时间：%v", stageName, time.Now())
	task()
}

func executeTrades(g *config.GlobalVars, phone string, products map[string]string, uid string, client *http.Client) {
	for title, aid := range products {
		if isAlreadyTraded(g, title, phone) {
			log.Printf("[Skip] %s %s 已兑换", phone, title)
			continue
		}
		if isWaitingTooLong(g.Wt) {
			log.Println("[Timeout] 等待时间超过30分钟，退出")
			return
		}
		log.Printf("[Trade] phone=%s item=%s", phone, title)
		// 每笔交易单独开启 goroutine 执行
		go exchange.Dh(g, phone, title, aid, g.Wt, uid, client)
	}
}

func isAlreadyTraded(g *config.GlobalVars, title, phone string) bool {
	if _, ok := g.Dhjl[g.Yf][title]; !ok {
		g.Dhjl[g.Yf][title] = ""
		return false
	}
	return strings.Contains(g.Dhjl[g.Yf][title], phone)
}

func isWaitingTooLong(targetTime float64) bool {
	return targetTime-float64(time.Now().Unix()) > 1800
}

// waitUntilTargetTime 等待直到目标时间到达
func waitUntilTargetTime(targetTime float64) {
	for {
		if float64(time.Now().Unix()) >= targetTime {
			break
		}
		time.Sleep(time.Second)
	}
}

func handleExchangeLog(g *config.GlobalVars) {
	nowMonth := time.Now().Format("200601")
	dhjl2 := make(map[string]map[string][]string)

	if oldLog := g.Dhjl[nowMonth]; oldLog != nil {
		for fee, phones := range oldLog {
			phones = strings.Trim(phones, "#")
			if phones == "" {
				continue
			}
			for _, phone := range strings.Split(phones, "#") {
				if phone == "" {
					continue
				}
				if _, ok := dhjl2[phone]; !ok {
					dhjl2[phone] = make(map[string][]string)
				}
				dhjl2[phone][nowMonth] = append(dhjl2[phone][nowMonth], fee)
			}
		}
	}

	data, _ := json.MarshalIndent(dhjl2, "", "  ")
	_ = os.WriteFile(config.ExchangeLogFile2, data, 0644)
}
