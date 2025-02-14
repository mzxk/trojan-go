//go:build custom || full
// +build custom full

package build

import (
	_ "github.com/mzxk/trojan-go/proxy/custom"
	_ "github.com/mzxk/trojan-go/tunnel/adapter"
	_ "github.com/mzxk/trojan-go/tunnel/dokodemo"
	_ "github.com/mzxk/trojan-go/tunnel/freedom"
	_ "github.com/mzxk/trojan-go/tunnel/http"
	_ "github.com/mzxk/trojan-go/tunnel/mux"
	_ "github.com/mzxk/trojan-go/tunnel/router"
	_ "github.com/mzxk/trojan-go/tunnel/shadowsocks"
	_ "github.com/mzxk/trojan-go/tunnel/simplesocks"
	_ "github.com/mzxk/trojan-go/tunnel/socks"
	_ "github.com/mzxk/trojan-go/tunnel/tls"
	_ "github.com/mzxk/trojan-go/tunnel/tproxy"
	_ "github.com/mzxk/trojan-go/tunnel/transport"
	_ "github.com/mzxk/trojan-go/tunnel/trojan"
	_ "github.com/mzxk/trojan-go/tunnel/websocket"
)
