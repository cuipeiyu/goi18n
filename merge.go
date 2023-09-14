package main

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	i18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

func merge() {
	defaultLang := viper.GetString("default")
	if defaultLang == "" {
		log.Error("default language is empty")
		return
	}

	targetLang := viper.GetStringSlice("target")
	if targetLang == nil {
		log.Error("target languages is empty")
		return
	}

	for _, lang := range targetLang {
		mergeLang(defaultLang, lang)
	}
}

func mergeLang(original, target string) {
	workspace := filepath.Dir(getGoEnv("GOMOD"))
	outdir := viper.GetString("outdir")
	outformat := viper.GetString("outformat")

	originalMap := make(M)
	todoMap := make(M)
	targetMap := make(M)

	localedir := filepath.Join(workspace, outdir)

	originalFilename := filepath.Join(localedir, original+"."+outformat)

	todoFilename := filepath.Join(localedir, target+".todo."+outformat)

	targetFilename := filepath.Join(localedir, target+"."+outformat)
	targetSignFilename := filepath.Join(localedir, target+".sign")

	// read original
	{
		if _, err := os.Stat(originalFilename); os.IsNotExist(err) {
			log.Error("来源文件不存在")
			return
		}
		buf, err := os.ReadFile(originalFilename)
		if err != nil {
			log.Error("读取文件失败", zap.Error(err))
		}

		switch outformat {
		case "json":
			err = json.Unmarshal(buf, &originalMap)
		default:
			err = yaml.Unmarshal(buf, &originalMap)
		}
		if err != nil {
			log.Warn("无法识别的文件", zap.String("format", outformat), zap.Error(err))
		}

		if len(originalMap) == 0 {
			log.Info("无数据，跳过")
			// TODO 删除文件，不要无用的空文件？
			return
		}

		// 针对源文件 始终重新生成 sign 文件
		for _, item := range originalMap {
			item.Hash = hash(*item)
		}
	}

	// .todo file
	{
		if _, err := os.Stat(todoFilename); os.IsNotExist(err) {
			// log.Println("[INFO] 中间文件不存在")
		} else {
			buf, err := os.ReadFile(todoFilename)
			if err == nil {
				switch outformat {
				case "json":
					err = json.Unmarshal(buf, &todoMap)
				default:
					err = yaml.Unmarshal(buf, &todoMap)
				}
				if err != nil {
					log.Fatal("无法识别的文件", zap.Error(err))
				}
			}
		}
		// 计算hash
		for _, item := range todoMap {
			item.Hash = hash(*item)
		}
	}

	// target
	{
		if _, err := os.Stat(targetFilename); os.IsNotExist(err) {
			log.Debug("目标文件不存在", zap.String("path", targetFilename))
			// 复制来源文件与签名文件
			copyfile(originalFilename, targetFilename)
			copyfile(originalFilename, todoFilename)

			originalMap.writeSign(localedir, target)
			return
		}

		// 签名文件不存在时，无法完成比对
		if info, err := os.Stat(targetSignFilename); os.IsNotExist(err) || info.Size() == 0 {
			log.Debug("签名文件不存在", zap.String("path", targetSignFilename))
			// 覆盖来源文件与签名文件
			copyfile(originalFilename, targetFilename)
			copyfile(originalFilename, todoFilename)

			originalMap.writeSign(localedir, target)
			return
		}

		buf, err := os.ReadFile(targetFilename)
		if err != nil {
			log.Fatal("读取文件失败", zap.Error(err))
			return
		}

		switch outformat {
		case "json":
			err = json.Unmarshal(buf, &targetMap)
		default:
			err = yaml.Unmarshal(buf, &targetMap)
		}
		if err != nil {
			log.Fatal("无法识别的文件", zap.Error(err))
		}

		if len(targetMap) > 0 {
			// 合并 sign 文件
			buf, err := os.ReadFile(targetSignFilename)
			if err != nil {
				log.Fatal("读取Sign文件失败", zap.Error(err))
			}
			// 合并
			signRecords := make(map[string]string)
			// 逐行读取
			scanner := bufio.NewScanner(bytes.NewReader(buf))
			for scanner.Scan() {
				line := scanner.Text()
				// 分隔
				parts := strings.Split(line, ":")
				if len(parts) == 2 {
					signRecords[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
				}
			}
			for rid, item := range targetMap {
				if h, ok := signRecords[rid]; ok {
					item.Hash = h
				}
			}
		}
	}

	if isDev {
		i, err := json.Marshal(originalMap)
		println("originalMap", string(i), err)

		j, err := json.Marshal(targetMap)
		println("targetMap", string(j), err)
	}

	// 比对文件
	todoMapResult, targetMapResult := diff(originalMap, todoMap, targetMap)

	// 写入
	{
		if len(targetMapResult) > 0 {
			targetMapResult.write2File(localedir, target)
			targetMapResult.writeSign(localedir, target)
		}
	}
	{
		if len(todoMapResult) > 0 {
			todoMapResult.write2File(localedir, target+".todo")
		} else {
			filename := filepath.Join(localedir, target+".todo."+outformat)
			_ = os.Remove(filename)
		}
	}
}

func diff(originalMap, middleMap, targetMap M) (M, M) {
	middleMapResult,
		targetMapResult :=
		make(M),
		make(M)

	for id, org := range originalMap {
		mid, midHas := middleMap[id]
		tar, tarHas := targetMap[id]

		// 需要翻译
		if !tarHas {
			middleMapResult[id] = org
			targetMapResult[id] = org
			continue
		}

		// 在todo文件中存在 && 是否已翻译 => 已翻译
		if midHas && hash(*mid) != hash(*tar) {
			targetMapResult[id] = mid
			targetMapResult[id].Hash = tar.Hash // 不改变
			continue
		}
		// 需要翻译
		if midHas && hash(*mid) == hash(*tar) {
			middleMapResult[id] = org
			targetMapResult[id] = org
			continue
		}

		// 存在 && 源内容已变更 => 重新翻译
		if tarHas && org.Hash != tar.Hash {
			middleMapResult[id] = tar
			continue
		}

		targetMapResult[id] = tar
	}

	// 	// 移除已被删除的
	// root1:
	// 	for id := range middleMapResult {
	// 		for oid := range originalMap {
	// 			if oid == id {
	// 				continue root1
	// 			}
	// 		}
	// 		// 不存在，移除
	// 		delete(middleMapResult, id)
	// 	}

	// 	// 移除已被删除的
	// root2:
	// 	for id := range targetMapResult {
	// 		for oid := range originalMap {
	// 			if oid == id {
	// 				continue root2
	// 			}
	// 		}
	// 		// 不存在，移除
	// 		delete(targetMapResult, id)
	// 	}

	return middleMapResult, targetMapResult
}

func hash(t i18n.Message) string {
	h := sha1.New()
	// _, _ = io.WriteString(h, t.Description)
	_, _ = io.WriteString(h, t.Zero)
	_, _ = io.WriteString(h, t.One)
	_, _ = io.WriteString(h, t.Two)
	_, _ = io.WriteString(h, t.Few)
	_, _ = io.WriteString(h, t.Many)
	_, _ = io.WriteString(h, t.Other)
	return hex.EncodeToString(h.Sum(nil))
}
