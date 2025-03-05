package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
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
		go func(ac string) {
			defer wg.Done()
			processAccount(ac, g, client, cfg)
		}(account)
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

func processAccount(accountStr string, g *config.GlobalVars, client *http.Client, cfg *config.Config) {
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
		if vm, ok := v.(map[string]interface{}); ok {
			if userId, ok1 := vm["userId"].(string); ok1 {
				if token, ok2 := vm["token"].(string); ok2 {
					if tk, err := sign.GetTicket(phone, userId, token); err == nil {
						return tk
					}
				}
			}
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

	// 执行预热阶段（3秒前抢发、1秒前实际预热）
	phoneNum, _ := strconv.ParseInt(phone, 10, 32)
	executeWarmupStages(g, int32(phoneNum), titles, aids, client, targetTime)

	// 执行实际交易
	var tradeWg sync.WaitGroup
	for title, aid := range products {
		// 修正：传递 int32(phoneNum) 而非 fmt.Sprint(phoneNum)
		if isAlreadyTraded(g, title, int32(phoneNum)) {
			log.Printf("[Skip] %d %s 已兑换", phoneNum, title)
			continue
		}
		// 如果距离目标时间超过30秒，放弃
		if isWaitingTooLong(targetTime) {
			log.Println("[Timeout] 等待时间超过30秒，退出")
			return
		}
		log.Printf("[Trade] phone=%d item=%s", phoneNum, title)

		tradeWg.Add(1)
		go func(t, a, u string) {
			defer tradeWg.Done()
			exchange.Dh(g, phone, t, a, targetTime, u, client)
		}(title, aid, uid)
	}
	tradeWg.Wait()
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
func executeWarmupStages(g *config.GlobalVars, phone int32, titles, aids []string, client *http.Client, targetTime float64) {
	var wg sync.WaitGroup

	// 基准时间
	baseTime := time.Unix(int64(targetTime), 0)
	emptyRequestTime := baseTime.Add(-3 * time.Second)
	realRequestTime := baseTime.Add(-1 * time.Second)

	// 阶段1：3秒前的空请求抢发
	if time.Now().Before(emptyRequestTime) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			scheduleStage(emptyRequestTime, "抢发阶段", func() {
				exchange.DoHighFreqRequests(emptyRequestTime, fmt.Sprint(phone), client, nil)
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
				exchange.DoHighFreqRealRequests(realRequestTime, fmt.Sprint(phone), titles, aids, client, nil)
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

// isAlreadyTraded 使用切片来检查是否已包含目标手机号
func isAlreadyTraded(g *config.GlobalVars, title string, phone int32) bool {
	phones, ok := g.Dhjl[g.Yf][title]
	if !ok {
		// 说明当前title还没有任何兑换记录, 初始化为空切片
		g.Dhjl[g.Yf][title] = []string{}
		return false
	}
	phoneStr := fmt.Sprint(phone)
	// 遍历切片检查是否存在
	for _, p := range phones {
		if p == phoneStr {
			return true
		}
	}
	return false
}

// isWaitingTooLong 当当前时间与目标时间差大于30秒时返回true
func isWaitingTooLong(targetTime float64) bool {
	return float64(time.Now().Unix())-targetTime > 30
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

	if oldLog, ok := g.Dhjl[nowMonth]; ok {
		for fee, phones := range oldLog {
			for _, phone := range phones {
				if phone == "" {
					continue
				}
				phoneStr := phone
				if _, ok2 := dhjl2[phoneStr]; !ok2 {
					dhjl2[phoneStr] = make(map[string][]string)
				}
				dhjl2[phoneStr][nowMonth] = append(dhjl2[phoneStr][nowMonth], fee)
			}
		}
	}

	data, _ := json.MarshalIndent(dhjl2, "", "  ")
	_ = os.WriteFile(config.ExchangeLogFile2, data, 0644)
}
