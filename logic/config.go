package logic

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Config 结构体用于解析JSON和TOML配置
type Config struct {
	Rules []*Rule `json:"rules" toml:"rules"`
}

// Rule 结构体表示一条规则
type Rule struct {
	File    string   `json:"file" toml:"file"`
	Imports []Import `json:"imports" toml:"imports"`
	Structs []Struct `json:"structs" toml:"structs"`
}

// Import 结构体表示导入信息
type Import struct {
	Path  string `json:"path" toml:"path"`
	Alias string `json:"alias" toml:"alias"`
}

// Struct 结构体表示结构体信息
type Struct struct {
	Name   string  `json:"name" toml:"name"`
	Fields []Field `json:"fields" toml:"fields"`
}

// Field 结构体表示字段信息
type Field struct {
	Name string `json:"name" toml:"name"`
	Type string `json:"type" toml:"type"`
	Tags string `json:"tags" toml:"tags"`
}

// ParseTOML 从TOML文件解析配置
func ParseTOML(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("无法打开TOML文件: %v", err)
	}
	defer file.Close()

	var config Config
	if _, err := toml.DecodeReader(file, &config); err != nil {
		return nil, fmt.Errorf("解析TOML文件失败: %v", err)
	}

	return &config, nil
}
