// Package socks 实现了 SOCKS5 代理服务器功能
package socks

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/mzxk/trojan-go/common"
	"github.com/mzxk/trojan-go/config"
	"github.com/mzxk/trojan-go/log"
	"github.com/mzxk/trojan-go/tunnel"
)

// SOCKS5 命令常量定义
const (
	// Connect 表示 SOCKS5 的 CONNECT 命令（连接到目标服务器）
	Connect tunnel.Command = 1
	// Associate 表示 SOCKS5 的 UDP ASSOCIATE 命令（建立 UDP 中继）
	Associate tunnel.Command = 3
)

// 其他常量定义
const (
	// MaxPacketSize 定义了 UDP 数据包的最大大小（8KB）
	MaxPacketSize = 1024 * 8
)

// Server 实现了 SOCKS5 服务器
type Server struct {
	// TCP 连接通道
	connChan chan tunnel.Conn
	// UDP 连接通道
	packetChan chan tunnel.PacketConn
	// 底层服务器接口
	underlay tunnel.Server
	// 服务器监听地址
	localHost string
	// 服务器监听端口
	localPort int
	// UDP 会话超时时间
	timeout time.Duration
	// UDP 监听连接
	listenPacketConn tunnel.PacketConn
	// UDP 会话映射表
	mapping map[string]*PacketConn
	// 映射表的读写锁
	mappingLock sync.RWMutex
	// 上下文和取消函数
	ctx    context.Context
	cancel context.CancelFunc
}

// AcceptConn 接受新的 TCP 连接
func (s *Server) AcceptConn(tunnel.Tunnel) (tunnel.Conn, error) {
	select {
	case conn := <-s.connChan:
		return conn, nil
	case <-s.ctx.Done():
		return nil, common.NewError("socks server closed")
	}
}

// AcceptPacket 接受新的 UDP 连接
func (s *Server) AcceptPacket(tunnel.Tunnel) (tunnel.PacketConn, error) {
	select {
	case conn := <-s.packetChan:
		return conn, nil
	case <-s.ctx.Done():
		return nil, common.NewError("socks server closed")
	}
}

// Close 关闭服务器
func (s *Server) Close() error {
	s.cancel()
	return s.underlay.Close()
}

// handshake 处理 SOCKS5 协议的握手过程
func (s *Server) handshake(conn net.Conn) (*Conn, error) {
	// 读取 SOCKS 版本号
	version := [1]byte{}
	if _, err := conn.Read(version[:]); err != nil {
		return nil, common.NewError("failed to read socks version").Base(err)
	}
	if version[0] != 5 {
		return nil, common.NewError(fmt.Sprintf("invalid socks version %d", version[0]))
	}

	// 读取认证方法数量
	nmethods := [1]byte{}
	if _, err := conn.Read(nmethods[:]); err != nil {
		return nil, common.NewError("failed to read NMETHODS")
	}

	// 读取支持的认证方法列表
	methods := make([]byte, nmethods[0])
	if _, err := conn.Read(methods); err != nil {
		return nil, common.NewError("failed to read methods").Base(err)
	}

	// 检查是否需要用户名密码认证
	cfg := config.FromContext(s.ctx, Name).(*Config)
	if cfg.Username != "" && cfg.Password != "" {
		// 发送需要认证的响应
		if _, err := conn.Write([]byte{0x05, 0x02}); err != nil {
			return nil, common.NewError("failed to respond auth method").Base(err)
		}

		// 读取认证协议版本
		authVer := [1]byte{}
		if _, err := conn.Read(authVer[:]); err != nil {
			return nil, common.NewError("failed to read auth version").Base(err)
		}
		if authVer[0] != 0x01 {
			return nil, common.NewError("unsupported auth version")
		}

		// 读取用户名长度和用户名
		ulen := [1]byte{}
		if _, err := conn.Read(ulen[:]); err != nil {
			return nil, common.NewError("failed to read username length").Base(err)
		}
		username := make([]byte, ulen[0])
		if _, err := conn.Read(username); err != nil {
			return nil, common.NewError("failed to read username").Base(err)
		}

		// 读取密码长度和密码
		plen := [1]byte{}
		if _, err := conn.Read(plen[:]); err != nil {
			return nil, common.NewError("failed to read password length").Base(err)
		}
		password := make([]byte, plen[0])
		if _, err := conn.Read(password); err != nil {
			return nil, common.NewError("failed to read password").Base(err)
		}

		// 验证用户名和密码
		if string(username) != cfg.Username || string(password) != cfg.Password {
			conn.Write([]byte{0x01, 0x01}) // 认证失败
			return nil, common.NewError("invalid credentials")
		}

		// 发送认证成功响应
		if _, err := conn.Write([]byte{0x01, 0x00}); err != nil {
			return nil, common.NewError("failed to send auth response").Base(err)
		}
	} else {
		// 不需要认证，发送无需认证的响应
		if _, err := conn.Write([]byte{0x05, 0x00}); err != nil {
			return nil, common.NewError("failed to respond auth").Base(err)
		}
	}

	// 读取客户端请求
	buf := [3]byte{}
	if _, err := conn.Read(buf[:]); err != nil {
		return nil, common.NewError("failed to read command")
	}

	// 读取目标地址
	addr := new(tunnel.Address)
	if err := addr.ReadFrom(conn); err != nil {
		return nil, err
	}

	// 返回新的连接对象
	return &Conn{
		metadata: &tunnel.Metadata{
			Command: tunnel.Command(buf[1]),
			Address: addr,
		},
		Conn: conn,
	}, nil
}

// connect 处理 CONNECT 命令的响应
func (s *Server) connect(conn net.Conn) error {
	_, err := conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	return err
}

// associate 处理 UDP ASSOCIATE 命令的响应
func (s *Server) associate(conn net.Conn, addr *tunnel.Address) error {
	buf := bytes.NewBuffer([]byte{0x05, 0x00, 0x00})
	common.Must(addr.WriteTo(buf))
	_, err := conn.Write(buf.Bytes())
	return err
}

// packetDispatchLoop 处理 UDP 数据包的转发
func (s *Server) packetDispatchLoop() {
	for {
		// 创建缓冲区接收 UDP 数据
		buf := make([]byte, MaxPacketSize)
		n, src, err := s.listenPacketConn.ReadFrom(buf)
		if err != nil {
			select {
			case <-s.ctx.Done():
				log.Debug("exiting")
				return
			default:
				continue
			}
		}
		log.Debug("socks recv udp packet from", src)

		// 查找或创建 UDP 会话
		s.mappingLock.RLock()
		conn, found := s.mapping[src.String()]
		s.mappingLock.RUnlock()

		if !found {
			// 创建新的 UDP 会话
			ctx, cancel := context.WithCancel(s.ctx)
			conn = &PacketConn{
				input:      make(chan *packetInfo, 128),
				output:     make(chan *packetInfo, 128),
				ctx:        ctx,
				cancel:     cancel,
				PacketConn: s.listenPacketConn,
				src:        src,
			}

			// 启动 UDP 转发协程
			go func(conn *PacketConn) {
				defer conn.Close()
				for {
					select {
					case info := <-conn.output:
						// 构建 UDP 响应数据包
						buf := bytes.NewBuffer(make([]byte, 0, MaxPacketSize))
						buf.Write([]byte{0, 0, 0}) // RSV, FRAG
						common.Must(info.metadata.Address.WriteTo(buf))
						buf.Write(info.payload)

						// 发送数据包
						_, err := s.listenPacketConn.WriteTo(buf.Bytes(), conn.src)
						if err != nil {
							log.Error("socks failed to respond packet to", src)
							return
						}
						log.Debug("socks respond udp packet to", src, "metadata", info.metadata)

					// UDP 会话超时处理
					case <-time.After(time.Second * 5):
						log.Info("socks udp session timeout, closed")
						s.mappingLock.Lock()
						delete(s.mapping, src.String())
						s.mappingLock.Unlock()
						return

					// 会话关闭处理
					case <-conn.ctx.Done():
						log.Info("socks udp session closed")
						return
					}
				}
			}(conn)

			// 将新会话加入映射表
			s.mappingLock.Lock()
			s.mapping[src.String()] = conn
			s.mappingLock.Unlock()

			s.packetChan <- conn
			log.Info("socks new udp session from", src)
		}

		// 解析 UDP 数据包
		r := bytes.NewBuffer(buf[3:n])
		address := new(tunnel.Address)
		if err := address.ReadFrom(r); err != nil {
			log.Error(common.NewError("socks failed to parse incoming packet").Base(err))
			continue
		}

		// 读取负载数据
		payload := make([]byte, MaxPacketSize)
		length, _ := r.Read(payload)

		// 发送到处理通道
		select {
		case conn.input <- &packetInfo{
			metadata: &tunnel.Metadata{
				Address: address,
			},
			payload: payload[:length],
		}:
		default:
			log.Warn("socks udp queue full")
		}
	}
}

// acceptLoop 处理新的客户端连接
func (s *Server) acceptLoop() {
	for {
		// 接受新连接
		conn, err := s.underlay.AcceptConn(&Tunnel{})
		if err != nil {
			log.Error(common.NewError("socks accept err").Base(err))
			return
		}

		// 为每个连接启动一个处理协程
		go func(conn net.Conn) {
			// 进行 SOCKS5 握手
			newConn, err := s.handshake(conn)
			if err != nil {
				log.Error(common.NewError("socks failed to handshake with client").Base(err))
				return
			}
			log.Info("socks connection from", conn.RemoteAddr(), "metadata", newConn.metadata.String())

			// 根据命令类型处理请求
			switch newConn.metadata.Command {
			case Connect:
				// 处理 CONNECT 命令
				if err := s.connect(newConn); err != nil {
					log.Error(common.NewError("socks failed to respond CONNECT").Base(err))
					newConn.Close()
					return
				}
				s.connChan <- newConn
				return
			case Associate:
				// 处理 UDP ASSOCIATE 命令
				defer newConn.Close()
				associateAddr := tunnel.NewAddressFromHostPort("udp", s.localHost, s.localPort)
				if err := s.associate(newConn, associateAddr); err != nil {
					log.Error(common.NewError("socks failed to respond to associate request").Base(err))
					return
				}
				buf := [16]byte{}
				newConn.Read(buf[:])
				log.Debug("socks udp session ends")
			default:
				log.Error(common.NewError(fmt.Sprintf("unknown socks command %d", newConn.metadata.Command)))
				newConn.Close()
			}
		}(conn)
	}
}

// NewServer 创建一个新的 SOCKS5 服务器
func NewServer(ctx context.Context, underlay tunnel.Server) (tunnel.Server, error) {
	// 获取配置
	cfg := config.FromContext(ctx, Name).(*Config)

	// 创建 UDP 监听
	listenPacketConn, err := underlay.AcceptPacket(&Tunnel{})
	if err != nil {
		return nil, common.NewError("socks failed to listen packet from underlying server")
	}

	// 创建服务器上下文
	ctx, cancel := context.WithCancel(ctx)

	// 初始化服务器
	server := &Server{
		underlay:         underlay,
		ctx:              ctx,
		cancel:           cancel,
		connChan:         make(chan tunnel.Conn, 32),
		packetChan:       make(chan tunnel.PacketConn, 32),
		localHost:        cfg.LocalHost,
		localPort:        cfg.LocalPort,
		timeout:          time.Duration(cfg.UDPTimeout) * time.Second,
		listenPacketConn: listenPacketConn,
		mapping:          make(map[string]*PacketConn),
	}

	// 启动接受连接和数据包处理的协程
	go server.acceptLoop()
	go server.packetDispatchLoop()

	log.Debug("socks server created")
	return server, nil
}
