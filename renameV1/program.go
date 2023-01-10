package main

func CreateConfigFile(param *Param) error {
	exist, err := IsConfigExist(param.WorkDir, param.ConfigFileName)
	if err != nil {
		return err
	}
	if exist && !param.SkipDoubleCheck {
		if err := DoubleCheck("配置文件已存在，是否覆盖"); err != nil {
			return err
		}
	}

	demoConfig := GenDemoConfig(param.CreateConfigFile)
	return SaveToConfigFile(param.WorkDir, param.ConfigFileName, demoConfig)
}

func UpdateConfigFile(param *Param) error {
	config, err := LoadConfig(param.WorkDir, param.ConfigFileName)
	if err != nil {
		return err
	}

	for i, conf := range config.Configs {
		var tmpFilenames []string
		if tmpFilenames, err = ListDirFiles(param.WorkDir, conf.Postfix); err != nil {
			return err
		}
		config.Configs[i].FileNames = tmpFilenames
	}

	if !param.SkipDoubleCheck {
		if err := DoubleCheck("是否覆盖配置文件"); err != nil {
			return err
		}
	}

	return SaveToConfigFile(param.WorkDir, param.ConfigFileName, config)
}

func Rename(param *Param) error {
	config, err := LoadConfig(param.WorkDir, param.ConfigFileName)
	if err != nil {
		return err
	}

	var renameFileInfoList []*RenameFileInfo
	for _, conf := range config.Configs {
		res := GenTargetFilename(conf.FileNames, conf.Policy)
		renameFileInfoList = append(renameFileInfoList, res...)
	}

	// 检查重命名冲突
	if err = CheckFileNameDuplicate(param.WorkDir, renameFileInfoList); err != nil {
		return err
	}

	if !param.SkipDoubleCheck {
		if err = DoubleCheck(GenPreviewNotice(renameFileInfoList)); err != nil {
			return err
		}
	}

	return ExecuteRename(renameFileInfoList)
}
