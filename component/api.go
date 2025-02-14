//go:build api || full
// +build api full

package build

import (
	_ "github.com/mzxk/trojan-go/api/control"
	_ "github.com/mzxk/trojan-go/api/service"
)
