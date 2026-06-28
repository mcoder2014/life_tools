package main

import (
	"flag"
	"os"

	"github.com/sirupsen/logrus"
)

func main() {
	GlobalParam = GetProgramParam()

	logrus.Infof("Receive request: %+v", *GlobalParam)
	Run()
}

func Run() {
	var err error

	var programs = []func(param *Param) error{
		func(param *Param) error {
			if GlobalParam.CreateConfigFile > 0 {
				return CreateConfigFile(GlobalParam)
			}
			return nil
		},
		func(param *Param) error {
			if GlobalParam.UpdateConfigFile {
				return UpdateConfigFile(GlobalParam)
			}
			return nil
		},
		func(param *Param) error {
			if GlobalParam.Rename {
				return Rename(GlobalParam)
			}
			return nil
		},
	}

	for _, program := range programs {
		if err = program(GlobalParam); err != nil {
			logrus.Errorf("Run program error: %v", err)
			return
		}
	}
	logrus.Infof("Run program success")
}

func GetProgramParam() *Param {
	var param Param

	flag.StringVar(&param.WorkDir, "dir", "./", "工作路径")
	flag.StringVar(&param.ConfigFileName, "conf_file_name", "rename_v1.json", "工作配置文件的文件名称")
	flag.IntVar(&param.CreateConfigFile, "create_conf", 0, "创建配置文件 层级数量")
	flag.BoolVar(&param.UpdateConfigFile, "update_conf", false, "更新配置文件")
	flag.BoolVar(&param.Rename, "rename", false, "依据配置文件执行重命名操作")
	flag.BoolVar(&param.SkipDoubleCheck, "skip_double_check", false, "跳过命令行二次确认")

	// 从arguments中解析注册的flag。必须在所有flag都注册好而未访问其值时执行。未注册却使用flag -help时，会返回ErrHelp。
	flag.Parse()

	if len(os.Args) == 1 {
		flag.Usage()
		os.Exit(1)
	}

	return &param
}
