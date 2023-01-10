package main

var GlobalParam *Param

type Param struct {
	// WorkDir 工作目录
	WorkDir string
	// ConfigFileName 指定配置文件的名称
	ConfigFileName string

	// 相互冲突的工作模式
	// 创建配置文件
	CreateConfigFile int
	// 更新配置文件
	UpdateConfigFile bool
	// 根据配置文件进行 rename
	Rename bool

	// SkipDoubleCheck 跳过命令行二次确认
	SkipDoubleCheck bool
}
