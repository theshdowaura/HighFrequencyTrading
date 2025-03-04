package cmd

import (
	"HighFrequencyTrading/config"
	"HighFrequencyTrading/util"
	"fmt"
	"github.com/spf13/cobra"
)

var (
	jdhfFlag     string
	mexzFlag     string
	hFlag        int
	useTradeHour bool

	rootCmd = &cobra.Command{
		Use:   "telecom",
		Short: "电信金豆换话费",
		// 无论是否指定子命令，这里的 PersistentPreRunE 都会先执行
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// 调用交易逻辑（如果需要使用交易时段参数，则传入 hFlag，否则传 nil）
			return RunMain(jdhfFlag, mexzFlag, useTradeHourToH())
		},
		// 如果没有子命令，则此处 RunE 不再重复执行交易逻辑
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
)

// useTradeHourToH 根据 useTradeHour 决定是否传递交易时段参数
func useTradeHourToH() *int {
	if useTradeHour {
		return &hFlag
	}
	return nil
}

// Execute 为命令执行入口
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&jdhfFlag, "jdhf", "", "电信账号信息,格式 phone#password#Uid(&phone2#pwd2#uid2)")
	rootCmd.PersistentFlags().StringVar(&mexzFlag, "mexz", "0.5,5,6;1,10,3", "兑换策略,如 0.5,5,6;1,10,3")
	// 将原 hf 参数重命名为 trade-hour，同时通过 use-trade-hour 决定是否使用
	rootCmd.PersistentFlags().IntVar(&hFlag, "trade-hour", 0, "交易时段: 9(上午场) 或 13(下午场)")
	rootCmd.PersistentFlags().BoolVar(&useTradeHour, "use-trade-hour", false, "是否启用交易时段参数")
	// 注册 wxpusher 子命令（注意：wxpusherCmd 定义在 pusher.go 中）
	rootCmd.AddCommand(wxpusherCmd)
}

// RunMain 真正执行主交易流程
func RunMain(cliJdhf, cliMEXZ string, cliH *int) error {
	var wxpusher util.Wxpusher // 此处仅用于展示 uid 配置，可根据需要调整
	cfg := config.NewConfig(cliJdhf, cliMEXZ, cliH)
	fmt.Printf("[Cobra] 最终配置: jdhf=%s, MEXZ=%s, trade-hour=%v, uid=%s\n",
		cfg.Jdhf, cfg.MEXZ, cfg.H, wxpusher.Uid)
	// 调用主交易逻辑（耗时流程）
	MainLogic(cfg)
	return nil
}
