// Package main 实现 protoc-gen-go-errors 插件，读取 servora.errors.v1.default_code / code 选项并校验 100..599 HTTP 状态码。
//
// 支持两个生成目标：
//   - target=go（缺省与显式参数保持字节级一致输出）：生成 *_errors.pb.go，仅依赖 github.com/go-kratos/kratos/v3/errors。
//   - target=ts：仅针对显式声明 Servora error 选项的顶层 enum 生成同路径 *.errors.ts，
//     通过 ./index.js 的 type-only import 复用联合类型，仅输出同名常量对象与 isXxx 类型守卫。
package main

import (
	"flag"
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
)

var version = "devel"

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("protoc-gen-go-errors %s\n", version)
		return
	}

	options, target := newGeneratorOptions()
	options.Run(func(plugin *protogen.Plugin) error {
		return generate(plugin, *target)
	})
}

func newGeneratorOptions() (protogen.Options, *string) {
	target := generatorTargetGo
	parameterFlags := flag.NewFlagSet("protoc-gen-go-errors", flag.ContinueOnError)
	parameterFlags.StringVar(&target, "target", generatorTargetGo, "generation target: go or ts")
	return protogen.Options{ParamFunc: parameterFlags.Set}, &target
}
