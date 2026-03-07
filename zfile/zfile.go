package zfile

import (
	"errors"
	"go/format"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
)

// ParseFile 获取文件信息
func ParseFile(file *multipart.FileHeader) (ext string, fileName string, size int64) {
	fullExt := filepath.Ext(file.Filename) // 含点号，如 ".jpg"
	ext = strings.TrimPrefix(fullExt, ".")
	fileName = strings.TrimSuffix(file.Filename, fullExt)
	size = file.Size
	return
}

// Exists 检查文件或目录是否存在
func Exists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, os.ErrNotExist)
}

// Mkdir 创建文件夹
func Mkdir(path string) error {
	return os.MkdirAll(path, os.ModePerm)
}

// CreateFile 创建文件
func CreateFile(filename, content string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	return err
}

func IsDirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// 读取最多1个条目（包含 . 和 ..）
	_, err = f.Readdir(1)

	if err == io.EOF {
		return true, nil // 没有条目，目录为空
	}

	return false, err // 有条目或发生错误
}

// GoFmt 格式化 Go 代码文件
func GoFmt(filename string) error {
	// 读取文件内容
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	// 格式化代码
	formatted, err := format.Source(content)
	if err != nil {
		return err
	}

	// 写回文件
	return os.WriteFile(filename, formatted, 0644)
}

// GoFmtDir 格式化目录下的所有 Go 代码文件
func GoFmtDir(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 只处理 .go 文件
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			if err := GoFmt(path); err != nil {
				// 忽略格式化错误，继续处理其他文件
				return nil
			}
		}

		return nil
	})
}
