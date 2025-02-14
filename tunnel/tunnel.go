// Package tunnel 实现了 Trojan-Go 的隧道功能
// 隧道是一个抽象的概念，它提供了网络连接的封装和转发功能
// 支持 TCP 连接和 UDP 数据包的处理
package tunnel

import (
	"context"
	"io"
	"net"

	"github.com/mzxk/trojan-go/common"
)

// Conn 是隧道中的 TCP 连接接口
// 它扩展了标准库的 net.Conn 接口，增加了元数据支持
type Conn interface {
	net.Conn
	Metadata() *Metadata
}

// PacketConn 是隧道中的 UDP 数据包连接接口
// 它扩展了标准库的 net.PacketConn 接口，增加了元数据支持
type PacketConn interface {
	net.PacketConn
	// WriteWithMetadata 写入带元数据的数据包
	WriteWithMetadata([]byte, *Metadata) (int, error)
	// ReadWithMetadata 读取带元数据的数据包
	ReadWithMetadata([]byte) (int, *Metadata, error)
}

// ConnDialer 用于在隧道中创建 TCP 连接
type ConnDialer interface {
	DialConn(*Address, Tunnel) (Conn, error)
}

// PacketDialer 用于在隧道中创建 UDP 数据包流
type PacketDialer interface {
	DialPacket(Tunnel) (PacketConn, error)
}

// ConnListener 用于接受 TCP 连接
type ConnListener interface {
	AcceptConn(Tunnel) (Conn, error)
}

// PacketListener 用于接受 UDP 数据包流
// 由于我们没有基于数据包流的隧道，所以 AcceptPacket 总是会接收到一个真实的 PacketConn
type PacketListener interface {
	AcceptPacket(Tunnel) (PacketConn, error)
}

// Dialer 是一个组合接口，可以同时建立 TCP 和 UDP 连接
type Dialer interface {
	ConnDialer
	PacketDialer
}

// Listener 是一个组合接口，可以同时接受 TCP 和 UDP 流
type Listener interface {
	ConnListener
	PacketListener
}

// Client 是基于流连接的隧道客户端接口
type Client interface {
	Dialer
	io.Closer
}

// Server 是基于流连接的隧道服务器接口
type Server interface {
	Listener
	io.Closer
}

// Tunnel 描述了一个隧道，允许从另一个隧道创建新的隧道
// 我们假设底层隧道完全了解上层隧道的工作方式，并且对上层隧道来说是透明的
type Tunnel interface {
	// Name 返回隧道的名称
	Name() string
	// NewClient 创建一个新的客户端
	NewClient(context.Context, Client) (Client, error)
	// NewServer 创建一个新的服务器
	NewServer(context.Context, Server) (Server, error)
}

// tunnels 存储了所有已注册的隧道
var tunnels = make(map[string]Tunnel)

// RegisterTunnel 通过隧道名称注册一个隧道
func RegisterTunnel(name string, tunnel Tunnel) {
	tunnels[name] = tunnel
}

// GetTunnel 通过名称获取一个已注册的隧道
func GetTunnel(name string) (Tunnel, error) {
	if t, ok := tunnels[name]; ok {
		return t, nil
	}
	return nil, common.NewError("未知的隧道名称: " + name)
}
