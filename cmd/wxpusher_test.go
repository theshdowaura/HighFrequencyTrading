package cmd

import (
	"HighFrequencyTrading/util"
	"os"
	"path/filepath"
	"testing"
)

func TestWxPusherCommand(t *testing.T) {
	// 创建临时测试目录
	tmpDir, err := os.MkdirTemp("", "wxpusher-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 保存当前工作目录
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// 切换到临时目录
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	// 测试完成后切换回原目录
	defer os.Chdir(currentDir)

	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "正常配置测试",
			yaml: `appToken: "test-token"
uid: "test-Uid"`,
			wantErr: false,
		},
		{
			name: "空配置测试",
			yaml: `appToken: ""
uid: ""`,
			wantErr: false,
		},
		{
			name: "错误格式测试",
			yaml: `appToken: test-token
uid: [invalid-yaml`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFile := filepath.Join(tmpDir, "wxpusher.yaml")

			// 写入测试配置
			err := os.WriteFile(configFile, []byte(tt.yaml), 0644)
			if err != nil {
				t.Fatal(err)
			}

			// 执行命令
			err = wxpusherCmd.Execute()

			// 验证结果
			if (err != nil) != tt.wantErr {
				t.Errorf("执行命令出错 = %v, 期望错误 = %v", err, tt.wantErr)
			}

			// 清理配置文件
			_ = os.Remove(configFile)
		})
	}
}

// 测试配置文件不存在的情况
func TestWxPusherCommandNoFile(t *testing.T) {
	// 创建临时测试目录
	tmpDir, err := os.MkdirTemp("", "wxpusher-test-nofile")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 切换到临时目录
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(currentDir)

	if err := wxpusherCmd.Execute(); err == nil {
		t.Error("期望文件不存在时返回错误，但没有")
	}
}

// 测试 Wxpusher 结构体
func TestWxPusherStruct(t *testing.T) {
	wxpusher := util.Wxpusher{
		AppToken: "test-token",
		Uid:      "test-Uid",
	}

	if wxpusher.AppToken == "" || wxpusher.Uid == "" {
		t.Error("Wxpusher 结构体初始化失败")
	}
}
