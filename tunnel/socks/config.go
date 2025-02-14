// Package socks 实现了 SOCKS5 代理协议
package socks

import "github.com/mzxk/trojan-go/config"

// Config 定义了 SOCKS5 服务器的配置结构
type Config struct {
	// LocalHost 是 SOCKS5 服务器监听的地址
	// 通常设置为 "127.0.0.1" 或 "0.0.0.0"
	LocalHost string `json:"local_addr" yaml:"local-addr"`

	// LocalPort 是 SOCKS5 服务器监听的端口
	// 常用端口为 1080
	LocalPort int `json:"local_port" yaml:"local-port"`

	// UDPTimeout 是 UDP 会话的超时时间（秒）
	// 如果 UDP 会话在此时间内没有活动，将被关闭
	UDPTimeout int `json:"udp_timeout" yaml:"udp-timeout"`

	// Username 是 SOCKS5 认证的用户名
	// 如果设置了用户名和密码，将启用认证功能
	Username string `json:"username" yaml:"username"`

	// Password 是 SOCKS5 认证的密码
	// 如果为空，则不启用认证
	Password string `json:"password" yaml:"password"`
}

// init 函数在包初始化时注册配置创建器
func init() {
	config.RegisterConfigCreator(Name, func() interface{} {
		return &Config{
			// 默认 UDP 超时时间为 60 秒
			UDPTimeout: 60,
		}
	})
}
