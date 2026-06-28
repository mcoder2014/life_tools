package main

import (
	"fmt"
	"io"
	"os"

	jsoniter "github.com/json-iterator/go"
)

type ConfigRenameApp struct {
	Configs []*ConfigRename `json:"configs"`
}

type ConfigRename struct {
	// 用来过滤文件名，在 update 时使用
	Postfix []string `json:"postfix"`
	// 新名称生成规则
	Policy *RenamePolicy `json:"policy"`
	// 文件排序列表，会根据这个列表进行重命名
	FileNames []string `json:"fileNames"`
}

type RenamePolicy struct {
	Name        string `json:"name"`
	NumStart    int    `json:"num_start"`
	NumInterval int    `json:"num_interval"`
	// RenameFilenameExtension 是否改变文件后缀名
	RenameFilenameExtension bool `json:"rename_filename_extension"`
	EPDigitCount            int  `json:"ep_digit_count"`
}

const (
	// PlaceholderEP 第几集的占位符
	PlaceholderEP = "{Ep}"
	// PlaceholderSkip 跳过某集的序号
	PlaceholderSkip = "skip"
)

func (c *ConfigRenameApp) Clone() *ConfigRenameApp {
	var res = &ConfigRenameApp{}
	for _, config := range c.Configs {
		res.Configs = append(res.Configs, config.Clone())
	}
	return res
}

func (c *ConfigRename) Clone() *ConfigRename {
	var res = &ConfigRename{
		Postfix: c.Postfix,
		Policy: &RenamePolicy{
			Name:                    c.Policy.Name,
			NumStart:                c.Policy.NumStart,
			NumInterval:             c.Policy.NumInterval,
			RenameFilenameExtension: c.Policy.RenameFilenameExtension,
			EPDigitCount:            c.Policy.EPDigitCount,
		},
	}
	for _, fileName := range c.FileNames {
		res.FileNames = append(res.FileNames, fileName)
	}
	return res
}

func GenDemoConfig(count int) *ConfigRenameApp {
	var demo = &ConfigRename{
		Postfix: []string{".mp4", ".mkv"},
		Policy: &RenamePolicy{
			Name:                    fmt.Sprintf("rename plicy.S1E%s.1080p", PlaceholderEP),
			NumStart:                1,
			NumInterval:             1,
			RenameFilenameExtension: false,
			EPDigitCount:            2,
		},
		FileNames: []string{
			"rename plicy.S1E01.1080p.mp4",
			"rename plicy.S1E02.1080p.mp4",
		},
	}

	var res = &ConfigRenameApp{}
	for i := 0; i < count; i++ {
		res.Configs = append(res.Configs, demo.Clone())
	}
	return res
}

func LoadConfig(dir, filename string) (*ConfigRenameApp, error) {
	isExist, _ := IsConfigExist(dir, filename)
	if !isExist {
		return nil, fmt.Errorf("config file not exist")
	}

	configPath := Join(dir, filename)
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	var res = &ConfigRenameApp{}
	err = jsoniter.Unmarshal(content, res)
	return res, err
}

func SaveToConfigFile(dir, filename string, config *ConfigRenameApp) error {
	jsonval, _ := jsoniter.MarshalIndent(config, "", "    ")

	filepath := Join(dir, filename)
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(jsonval)
	return err
}

func IsConfigExist(dir, file string) (bool, error) {
	configFilePath := Join(dir, file)
	return FileExist(configFilePath)
}
