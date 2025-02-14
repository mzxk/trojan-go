package server

import (
	"github.com/mzxk/trojan-go/config"
	"github.com/mzxk/trojan-go/proxy/client"
)

func init() {
	config.RegisterConfigCreator(Name, func() interface{} {
		return new(client.Config)
	})
}
