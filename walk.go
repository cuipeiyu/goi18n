package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	i18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

// 遍历项目文件夹找出代码片段
func extract() {
	log.Debug("开始遍历文件夹")

	workspace := filepath.Dir(getGoEnv("GOMOD"))
	ignoreTestFiles := viper.GetBool("ignore-test-files")

	paths := viper.GetStringSlice("path")
	if len(paths) == 0 {
		paths = []string{
			workspace,
		}
	}

	total := 0

	messages := []*i18n.Message{}
	for _, path := range paths {
		log.Info("搜索路径", zap.String("path", path))

		if err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if filepath.Ext(path) != ".go" {
				return nil
			}

			// Don't extract from test files.
			if ignoreTestFiles && strings.HasSuffix(path, "_test.go") {
				return nil
			}

			total++

			buf, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			msgs, err := extractMessages(buf)
			if err != nil {
				return err
			}
			messages = append(messages, msgs...)
			return nil
		}); err != nil {
			return
		}
	}

	if messages == nil {
		log.Info("无匹配数据")
		return
	}

	log.Info("处理文件数量", zap.Int("total", total))
	log.Info("找到翻译项", zap.Int("messages", len(messages)))

	// 准备翻译输出目录
	localedir := filepath.Join(workspace, viper.GetString("outdir"))
	if _, err := os.Stat(localedir); os.IsNotExist(err) {
		log.Debug("创建文件夹", zap.String("path", localedir))
		err := os.MkdirAll(localedir, os.ModeDir|os.ModePerm)
		if err != nil {
			log.Fatal("创建文件夹失败", zap.Error(err))
		}
	}
	log.Debug("翻译原文输出目录", zap.String("outdir", localedir))

	// // 准备翻译签名输出目录
	// signdir := filepath.Join(localedir, ".go-i18n")
	// if _, err := os.Stat(signdir); os.IsNotExist(err) {
	// 	log.Debug("创建文件夹", zap.String("path", signdir))
	// 	err := os.MkdirAll(signdir, os.ModeDir|os.ModePerm)
	// 	if err != nil {
	// 		log.Fatal("创建文件夹失败", zap.Error(err))
	// 	}
	// }
	// log.Debug("签名文件输出目录", zap.String("outdir", signdir))

	messageMap := make(M, len(messages))
	for _, item := range messages {
		item.Hash = hash(*item)
		messageMap[item.ID] = item
	}

	filename := viper.GetString("default")
	if err := messageMap.write2File(localedir, filename); err != nil {
		log.Fatal("写入文件出错", zap.Error(err))
		return
	}

	// 方便观察
	if isDev {
		if err := messageMap.writeSign(localedir, filename); err != nil {
			log.Fatal("写入文件出错", zap.Error(err))
			return
		}
	}
}

// extractMessages extracts messages from the bytes of a Go source file.
func extractMessages(buf []byte) ([]*i18n.Message, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", buf, parser.AllErrors)
	if err != nil {
		return nil, err
	}
	extractor := &extractor{i18nPackageName: i18nPackageName(file)}
	ast.Walk(extractor, file)
	return extractor.messages, nil
}
