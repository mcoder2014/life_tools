package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

func GenTargetFilename(filenames []string, policy *RenamePolicy) []*RenameFileInfo {
	var res []*RenameFileInfo

	// Generate
	for _, filename := range filenames {

		extension := filepath.Ext(filename)
		res = append(res, &RenameFileInfo{
			OldFullName: filename,
			extension:   extension,
		})
	}

	EpDigitCount := policy.EPDigitCount
	EpFormatStr := fmt.Sprintf("%%0%dd", EpDigitCount)

	// New
	var currentEp = policy.NumStart - 1
	for _, renameInfo := range res {
		// 先执行每次必要的动作
		currentEp += policy.NumInterval
		// 跳过不存在的集，使序号延续
		if strings.EqualFold(renameInfo.OldFullName, PlaceholderSkip) {
			continue
		}

		var extension = renameInfo.extension
		if policy.RenameFilenameExtension {
			extension = ""
		}
		newBaseName := strings.ReplaceAll(policy.Name, PlaceholderEP, fmt.Sprintf(EpFormatStr, currentEp))
		renameInfo.tmpFullName = fmt.Sprintf("%s%s%s", newBaseName, getUUID(), extension)
		renameInfo.NewFullName = fmt.Sprintf("%s%s", newBaseName, extension)

	}

	return res
}

func getUUID() string {
	uid := uuid.New()
	return uid.String()
}

type RenameFileInfo struct {
	OldFullName string
	extension   string
	NewFullName string
	tmpFullName string
}

func GetOldFilenameMap(renameInfoList []*RenameFileInfo) map[string]bool {
	var res = make(map[string]bool)
	for _, renameInfo := range renameInfoList {
		res[renameInfo.OldFullName] = true
	}
	return res
}

func GenPreviewNotice(renameInfoList []*RenameFileInfo) string {
	var sb strings.Builder

	sb.WriteString("以下文件将被重命名：\n")

	for _, renameInfo := range renameInfoList {
		sb.WriteString(renameInfo.OldFullName)
		sb.WriteString(" -> ")
		sb.WriteString(renameInfo.NewFullName)
		sb.WriteString("\n")
	}
	sb.WriteString("是否继续？")

	return sb.String()
}

func CheckFileNameDuplicate(dir string, renameInfoList []*RenameFileInfo) error {
	originTotalFileList, err := ListDirFiles(dir, nil)
	if err != nil {
		return err
	}

	// 移除改名前的文件
	oldFilenameMap := GetOldFilenameMap(renameInfoList)
	var totalFileMap = make(map[string]bool)
	for _, filename := range originTotalFileList {
		if !oldFilenameMap[filename] {
			totalFileMap[filename] = true
		}
	}

	// 检查改名后的文件是否重复
	for _, renameInfo := range renameInfoList {
		if totalFileMap[renameInfo.NewFullName] {
			return fmt.Errorf("改名后产生文件名冲突：%s", renameInfo.NewFullName)
		}
		totalFileMap[renameInfo.NewFullName] = true
	}
	return nil
}

func ExecuteRename(renameFileInfoList []*RenameFileInfo) error {
	for _, renameInfo := range renameFileInfoList {
		err := os.Rename(renameInfo.OldFullName, renameInfo.tmpFullName)
		if err != nil {
			return err
		}
	}
	for _, renameInfo := range renameFileInfoList {
		err := os.Rename(renameInfo.tmpFullName, renameInfo.NewFullName)
		if err != nil {
			return err
		}
	}
	return nil
}
