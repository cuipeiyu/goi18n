package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	i18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type M map[string]*i18n.Message

func (m M) write2File(outdir, filename string) error {
	outformat := viper.GetString("outformat")
	fullname := filepath.Join(outdir, filename+"."+outformat)

	t := make(map[string]map[string]string)
	for k, v := range m {
		kv := make(map[string]string)
		if v.Zero != "" {
			kv["zero"] = v.Zero
		}
		if v.One != "" {
			kv["one"] = v.One
		}
		if v.Two != "" {
			kv["two"] = v.Two
		}
		if v.Few != "" {
			kv["few"] = v.Few
		}
		if v.Many != "" {
			kv["many"] = v.Many
		}
		if v.Other != "" {
			kv["other"] = v.Other
		}
		t[k] = kv
	}

	var out []byte
	var err error

	if len(t) > 0 {
		switch outformat {
		case "json":
			out, err = json.Marshal(t)

		default:
			out, err = yaml.Marshal(t)
		}

		if err != nil {
			log.Error("marshal error", zap.String("format", outformat), zap.Error(err))
			return err
		}
	}

	return os.WriteFile(fullname, out, os.ModePerm)
}

func (m M) writeSign(outdir, filename string) error {
	outformat := viper.GetString("outformat")
	fullname := filepath.Join(outdir, filename+".sign")

	tmp := make(map[string]string)
	for k, v := range m {
		tmp[k] = v.Hash
	}

	if len(tmp) == 0 {
		_ = os.Remove(fullname)
		return nil
	}

	var out []byte
	var err error

	switch outformat {
	case "json":
		out, err = json.Marshal(tmp)

	default:
		out, err = yaml.Marshal(tmp)
	}

	if err != nil {
		log.Error("marshal failed", zap.String("format", outformat), zap.Error(err))
		return err
	}

	return os.WriteFile(fullname, out, os.ModePerm)
}
