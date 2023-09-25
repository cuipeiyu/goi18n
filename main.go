package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var isDev bool = strings.Contains(os.Args[0], "/go-build")

var log, _ = zap.NewDevelopment()

func main() {
	var rootCmd = &cobra.Command{Use: "goi18n"}

	// global flags

	// verbose
	rootCmd.PersistentFlags().Bool("verbose", isDev, "啰嗦模式 default: false")
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	// default
	rootCmd.PersistentFlags().StringP("default", "d", "en-US", "默认语言 default: en-US")
	rootCmd.MarkFlagRequired("default")
	viper.BindPFlag("default", rootCmd.PersistentFlags().Lookup("default"))

	// outdir
	rootCmd.PersistentFlags().String("outdir", "./locales", "输出文件夹 default: $PWD/locales")
	viper.BindPFlag("outdir", rootCmd.PersistentFlags().Lookup("outdir"))

	// outformat
	rootCmd.PersistentFlags().String("outformat", "yaml", "输出格式 default: yaml")
	viper.BindPFlag("outformat", rootCmd.PersistentFlags().Lookup("outformat"))

	// sub command extract
	{
		cmd := &cobra.Command{
			Use:              "extract",
			Short:            "遍历项目文件夹找出翻译语句",
			Long:             "",
			TraverseChildren: true,
			Run: func(cmd *cobra.Command, args []string) {
				extract()
			},
		}
		cmd.Flags().StringArray("path", nil, "")
		viper.BindPFlag("path", cmd.Flags().Lookup("path"))

		cmd.Flags().Bool("ignore-test-files", true, "是否忽略 _test.go 文件")
		viper.BindPFlag("ignore-test-files", cmd.Flags().Lookup("ignore-test-files"))

		rootCmd.AddCommand(cmd)
	}

	// sub command merge
	{
		cmd := &cobra.Command{
			Use:              "merge",
			Short:            "遍历项目文件夹找出翻译语句",
			Long:             "",
			TraverseChildren: true,
			Run: func(cmd *cobra.Command, args []string) {
				merge()
			},
		}

		// target
		cmd.PersistentFlags().StringArrayP("target", "t", nil, "目标语言")
		cmd.MarkFlagRequired("target")
		viper.BindPFlag("target", cmd.PersistentFlags().Lookup("target"))

		rootCmd.AddCommand(cmd)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// writeConfigFile()
}

var goEnvCache = make(map[string]string)

func getGoEnv(key string) string {
	if val, ok := goEnvCache[key]; ok {
		return val
	}
	out, err := exec.Command("go", "env", key).Output()
	if err != nil {
		panic(err.Error())
	}
	goEnvCache[key] = string(out)
	return string(out)
}

func copyfile(from, to string) {
	src, err := os.Open(from)
	if err != nil {
		panic(err)
	}
	defer src.Close()

	dest, err := os.Create(to)
	if err != nil {
		panic(err)
	}
	defer dest.Close()

	_, err = io.Copy(dest, src)
	if err != nil {
		panic(err)
	}
}
