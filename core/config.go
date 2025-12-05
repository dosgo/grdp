package core

import (
	"encoding/json"
	"fmt"
	"os"
)

type AppConfig struct {
	IP       string `json:"ip"`
	Username string `json:"username"`
	Password string `json:"password"`
}

const configFileName = "mstsc_config.json"

// 2. 保存配置到 JSON 文件
func SaveConfig(cfg AppConfig) {
	data, err := json.MarshalIndent(cfg, "", "  ") // 格式化输出，方便阅读
	if err != nil {
		fmt.Println("Error marshalling config:", err)
		return
	}
	err = os.WriteFile(configFileName, data, 0644)
	if err != nil {
		fmt.Println("Error writing config file:", err)
	} else {
		fmt.Println("Config saved successfully.")
	}
}

// 3. 从 JSON 文件读取配置
func LoadConfig() AppConfig {
	var cfg AppConfig
	data, err := os.ReadFile(configFileName)
	if err != nil {
		// 如果文件不存在或读取失败，返回空配置即可，不影响程序启动
		fmt.Println("No config file found or error reading, starting with empty fields.")
		return cfg
	}

	err = json.Unmarshal(data, &cfg)
	if err != nil {
		fmt.Println("Error parsing config file:", err)
	}
	return cfg
}
