package cmd

import (
	"fmt"

	"HighFrequencyTrading/config"
	"github.com/spf13/cobra"
)

var (
	jdhfFlag string
	mexzFlag string
	hFlag    int
	useHFlag bool

	rootCmd = &cobra.Command{
		Use:   "telecom",
		Short: "电信金豆换话费",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !useHFlag {
				// 若没加 --useH，则忽略 hFlag
				return RunMain(jdhfFlag, mexzFlag, nil)
			}
			return RunMain(jdhfFlag, mexzFlag, &hFlag)
		},
	}
)

// Execute 入口
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&jdhfFlag, "jdhf", "", "电信账号信息,格式 phone#password#uid(&phone2#pwd2#uid2)")
	rootCmd.PersistentFlags().StringVar(&mexzFlag, "mexz", "", "兑换策略,如 0.5,5,6;1,10,3")
	rootCmd.PersistentFlags().IntVar(&hFlag, "h", 0, "9(上午场) 或 13(下午场)")
	rootCmd.PersistentFlags().BoolVar(&useHFlag, "useH", false, "是否启用 -h 参数")
	subCmd.Flags().StringVar(&content, "p", "", "微信推送器")
}

// RunMain 真正执行流程
func RunMain(cliJdhf, cliMEXZ string, cliH *int) error {
	cfg := config.NewConfig(cliJdhf, cliMEXZ, cliH)
	fmt.Printf("[Cobra] 最终配置: jdhf=%s, MEXZ=%s, h=%v\n",
		cfg.Jdhf, cfg.MEXZ, cfg.H)
	MainLogic(cfg)
	return nil
}
