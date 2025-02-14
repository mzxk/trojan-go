// Package main 是 Trojan-Go 代理服务器的入口点
// Trojan-Go 是 Trojan 协议的现代化实现，这是一个无法被探测的协议
// 可以帮助用户绕过 GFW (长城防火墙) 的封锁
package main

import (
	"flag"

	// 导入所有组件以进行注册
	// 使用空导入 "_" 是因为我们只需要组件的 init() 函数来注册自己
	_ "github.com/mzxk/trojan-go/component"
	"github.com/mzxk/trojan-go/log"
	"github.com/mzxk/trojan-go/option"
)

// main 函数是 Trojan-Go 的入口点
// 它处理命令行参数并初始化代理服务器
// 程序会尝试处理所有已注册的选项，直到其中一个成功为止
func main() {
	// 解析命令行参数
	flag.Parse()

	// 持续尝试处理选项，直到有一个成功
	for {
		// 获取下一个选项处理器
		h, err := option.PopOptionHandler()
		if err != nil {
			// 如果选项无效，程序终止
			log.Fatal("invalid options")
		}
		// 尝试处理当前选项
		err = h.Handle()
		if err == nil {
			// 如果成功处理了一个选项，退出循环
			break
		}
		// 如果处理失败，继续尝试下一个选项
	}
}
