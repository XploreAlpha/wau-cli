package cluster

import (
	"fmt"

	"github.com/wau/wau-cli/internal/client"
)

// Accessor 函数(由 cmd 包注入,避免 import cycle)。
var (
	getKernelAddr = func() string { return "http://localhost:18400" }
	getRole       = func() string { return "external_agent" }
)

// SetAccessors 由 cmd 包在 init() 调,注入 accessor(per auth.SetAccessors 同款 pattern)。
func SetAccessors(kernelAddr, role func() string) {
	getKernelAddr = kernelAddr
	getRole = role
}

// newClient 从注入的 accessor 拿 kernel addr + role 构造 client。
func newClient() *client.Client {
	return client.NewClient(client.Options{
		BaseURL: getKernelAddr(),
		Role:    getRole(),
	})
}

// exitCodeError 让 Cobra 退出指定 code(per P4.4 stack restart 的同款 pattern)。
type exitCodeError int

func (e exitCodeError) Error() string {
	return fmt.Sprintf("exit code %d", int(e))
}

func (exitCodeError) ExitCode() int { return int(exitCodeError(0)) }