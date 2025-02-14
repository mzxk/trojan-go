//go:build mysql || full || mini
// +build mysql full mini

package build

import (
	_ "github.com/mzxk/trojan-go/statistic/mysql"
)
