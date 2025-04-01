/**
  @author: Hanhai
  @desc: 嵌入式静态文件系统管理，提供读取和列出嵌入文件的功能
**/

package web

import (
	"fmt"
	"io"
	"io/fs"
)

// GetEmbeddedFile 从嵌入式文件系统读取文件内容
// 参数path是相对于嵌入根目录的路径，如 "static/img/favicon_32.ico"
func GetEmbeddedFile(path string) ([]byte, error) {
	file, err := staticFS.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开嵌入式文件失败: %w", err)
	}
	defer file.Close()

	// 检查文件信息
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("获取嵌入式文件信息失败: %w", err)
	}

	// 检查是否为目录
	if stat.IsDir() {
		return nil, fmt.Errorf("路径 %s 是一个目录，不是文件", path)
	}

	// 读取文件内容
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("读取嵌入式文件内容失败: %w", err)
	}

	return data, nil
}

// ListEmbeddedFiles 列出嵌入式文件系统中指定目录下的所有文件
func ListEmbeddedFiles(dir string) ([]string, error) {
	entries, err := fs.ReadDir(staticFS, dir)
	if err != nil {
		return nil, fmt.Errorf("读取嵌入式目录失败: %w", err)
	}

	var files []string
	for _, entry := range entries {
		name := entry.Name()
		path := dir + "/" + name

		if entry.IsDir() {
			// 递归读取子目录
			subFiles, err := ListEmbeddedFiles(path)
			if err != nil {
				return nil, err
			}
			files = append(files, subFiles...)
		} else {
			files = append(files, path)
		}
	}

	return files, nil
}
