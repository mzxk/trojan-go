// Package proxy 实现了 Trojan 代理的核心功能
// 它负责处理连接的中继和数据包的转发
package proxy

import (
	"context"
	"io"
	"math/rand"
	"net"
	"os"
	"strings"

	"github.com/p4gefau1t/trojan-go/common"
	"github.com/p4gefau1t/trojan-go/config"
	"github.com/p4gefau1t/trojan-go/log"
	"github.com/p4gefau1t/trojan-go/tunnel"
)

// Name 是代理模块的标识符
const Name = "PROXY"

// MaxPacketSize 定义了数据包的最大大小（8KB）
const (
	MaxPacketSize = 1024 * 8
)

// Proxy 结构体负责中继连接和数据包
// 它包含了多个源服务器和一个目标客户端
type Proxy struct {
	sources []tunnel.Server    // 源服务器列表
	sink    tunnel.Client      // 目标客户端
	ctx     context.Context    // 上下文，用于控制生命周期
	cancel  context.CancelFunc // 取消函数
}

// Run 启动代理服务
// 它会同时启动连接中继和数据包中继的循环
func (p *Proxy) Run() error {
	p.relayConnLoop()
	p.relayPacketLoop()
	<-p.ctx.Done()
	return nil
}

// Close 关闭代理服务
// 它会清理所有资源并关闭所有连接
func (p *Proxy) Close() error {
	p.cancel()
	p.sink.Close()
	for _, source := range p.sources {
		source.Close()
	}
	return nil
}

// relayConnLoop 处理 TCP 连接的中继
// 为每个源服务器启动一个 goroutine 来接受连接
func (p *Proxy) relayConnLoop() {
	for _, source := range p.sources {
		go func(source tunnel.Server) {
			for {
				// 接受新的入站连接
				inbound, err := source.AcceptConn(nil)
				if err != nil {
					select {
					case <-p.ctx.Done():
						log.Debug("正在退出")
						return
					default:
					}
					log.Error(common.NewError("接受连接失败").Base(err))
					continue
				}
				// 为每个连接启动一个新的 goroutine 处理
				go func(inbound tunnel.Conn) {
					defer inbound.Close()
					// 建立到目标的出站连接
					outbound, err := p.sink.DialConn(inbound.Metadata().Address, nil)
					if err != nil {
						log.Error(common.NewError("代理无法建立连接").Base(err))
						return
					}
					defer outbound.Close()
					errChan := make(chan error, 2)
					// 复制连接数据的辅助函数
					copyConn := func(a, b net.Conn) {
						_, err := io.Copy(a, b)
						errChan <- err
					}
					// 双向复制数据
					go copyConn(inbound, outbound)
					go copyConn(outbound, inbound)
					select {
					case err = <-errChan:
						if err != nil {
							log.Error(err)
						}
					case <-p.ctx.Done():
						log.Debug("正在关闭连接中继")
						return
					}
					log.Debug("连接中继结束")
				}(inbound)
			}
		}(source)
	}
}

// relayPacketLoop 处理 UDP 数据包的中继
// 为每个源服务器启动一个 goroutine 来处理数据包
func (p *Proxy) relayPacketLoop() {
	for _, source := range p.sources {
		go func(source tunnel.Server) {
			for {
				// 接受新的入站数据包连接
				inbound, err := source.AcceptPacket(nil)
				if err != nil {
					select {
					case <-p.ctx.Done():
						log.Debug("正在退出")
						return
					default:
					}
					log.Error(common.NewError("接受数据包失败").Base(err))
					continue
				}
				// 为每个数据包连接启动一个新的 goroutine 处理
				go func(inbound tunnel.PacketConn) {
					defer inbound.Close()
					// 建立到目标的出站数据包连接
					outbound, err := p.sink.DialPacket(nil)
					if err != nil {
						log.Error(common.NewError("代理无法建立数据包连接").Base(err))
						return
					}
					defer outbound.Close()
					errChan := make(chan error, 2)
					// 复制数据包的辅助函数
					copyPacket := func(a, b tunnel.PacketConn) {
						for {
							buf := make([]byte, MaxPacketSize)
							n, metadata, err := a.ReadWithMetadata(buf)
							if err != nil {
								errChan <- err
								return
							}
							if n == 0 {
								errChan <- nil
								return
							}
							_, err = b.WriteWithMetadata(buf[:n], metadata)
							if err != nil {
								errChan <- err
								return
							}
						}
					}
					// 双向复制数据包
					go copyPacket(inbound, outbound)
					go copyPacket(outbound, inbound)
					select {
					case err = <-errChan:
						if err != nil {
							log.Error(err)
						}
					case <-p.ctx.Done():
						log.Debug("正在关闭数据包中继")
					}
					log.Debug("数据包中继结束")
				}(inbound)
			}
		}(source)
	}
}

// NewProxy 创建一个新的代理实例
func NewProxy(ctx context.Context, cancel context.CancelFunc, sources []tunnel.Server, sink tunnel.Client) *Proxy {
	return &Proxy{
		sources: sources,
		sink:    sink,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Creator 是创建代理实例的函数类型
type Creator func(ctx context.Context) (*Proxy, error)

// creators 存储了所有已注册的代理创建器
var creators = make(map[string]Creator)

// RegisterProxyCreator 注册一个新的代理创建器
func RegisterProxyCreator(name string, creator Creator) {
	creators[name] = creator
}

// NewProxyFromConfigData 从配置数据创建新的代理实例
// 支持 JSON 和 YAML 格式的配置
func NewProxyFromConfigData(data []byte, isJSON bool) (*Proxy, error) {
	// 为每个代理实例创建唯一的上下文，以避免认证器重复
	ctx := context.WithValue(context.Background(), Name+"_ID", rand.Int())
	var err error
	if isJSON {
		ctx, err = config.WithJSONConfig(ctx, data)
		if err != nil {
			return nil, err
		}
	} else {
		ctx, err = config.WithYAMLConfig(ctx, data)
		if err != nil {
			return nil, err
		}
	}
	cfg := config.FromContext(ctx, Name).(*Config)
	create, ok := creators[strings.ToUpper(cfg.RunType)]
	if !ok {
		return nil, common.NewError("未知的代理类型: " + cfg.RunType)
	}
	// 设置日志级别
	log.SetLogLevel(log.LogLevel(cfg.LogLevel))
	if cfg.LogFile != "" {
		file, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, common.NewError("无法打开日志文件").Base(err)
		}
		log.SetOutput(file)
	}
	return create(ctx)
}
