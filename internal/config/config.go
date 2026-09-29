// Package config 负责应用配置的持久化。
//
// 配置文件位置：Windows 下为 %APPDATA%\FFmpegStudio\config.json
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const appDirName = "FFmpegStudio"

// Config 是持久化的应用配置。
type Config struct {
	// FFmpegPath 为空表示尚未指定，交给自动探测。
	FFmpegPath string `json:"ffmpegPath"`
	// FFprobePath 通常由 FFmpegPath 同目录推导。
	FFprobePath string `json:"ffprobePath"`
	// LastOutputDir 给输出路径一个合理默认值。
	LastOutputDir string `json:"lastOutputDir"`

	file string
}

// Dir 返回配置目录，不存在则创建。
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, appDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// CacheDir 返回临时缓存目录（预览代理、两遍编码的 passlog 等）。
func CacheDir(sub string) (string, error) {
	dir := filepath.Join(os.TempDir(), appDirName, sub)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// Load 读取配置。文件不存在或损坏时返回空配置而不是报错，
// 保证应用在任何情况下都能启动。
func Load() *Config {
	dir, err := Dir()
	if err != nil {
		return &Config{}
	}
	file := filepath.Join(dir, "config.json")

	data, err := os.ReadFile(file)
	if err != nil {
		return &Config{file: file}
	}

	cfg := &Config{file: file}
	if err := json.Unmarshal(data, cfg); err != nil {
		return &Config{file: file}
	}
	return cfg
}

// Save 写回配置。
func (c *Config) Save() error {
	if c.file == "" {
		dir, err := Dir()
		if err != nil {
			return err
		}
		c.file = filepath.Join(dir, "config.json")
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.file, data, 0o644)
}

// FilePath 返回配置文件路径，设置页展示用。
func (c *Config) FilePath() string {
	return c.file
}
