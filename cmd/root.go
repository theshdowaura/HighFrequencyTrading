package cmd

import (
	"HighFrequencyTrading/util"
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
	rootCmd.PersistentFlags().StringVar(&jdhfFlag, "jdhf", "", "电信账号信息,格式 phone#password#Uid(&phone2#pwd2#uid2)")
	rootCmd.PersistentFlags().StringVar(&mexzFlag, "mexz", "0.5,5,6;1,10,3", "兑换策略,如 0.5,5,6;1,10,3")
	rootCmd.PersistentFlags().IntVar(&hFlag, "hf", 0, "9(上午场) 或 13(下午场)")
	rootCmd.PersistentFlags().BoolVar(&useHFlag, "useH", false, "是否启用 -h 参数")
	subCmd.Flags().StringVar(&AppToken, "a", "", "wxpusher的apptoken")
	subCmd.Flags().StringVar(&Uid, "u", "", "wxpusher的uid的值")
	rootCmd.AddCommand(subCmd)
}

// RunMain 真正执行流程
func RunMain(cliJdhf, cliMEXZ string, cliH *int) error {
	var wxpusher util.Wxpusher
	cfg := config.NewConfig(cliJdhf, cliMEXZ, cliH)
	fmt.Printf("[Cobra] 最终配置: jdhf=%s, MEXZ=%s, h=%v,uid=%s\n",
		cfg.Jdhf, cfg.MEXZ, cfg.H, wxpusher.Uid)
	MainLogic(cfg)
	return nil
}
