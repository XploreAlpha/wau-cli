package agent

// getKernelAddr returns the kernel address from the global config
func getKernelAddr() string {
	return kernelAddrAccessor()
}

func getRole() string {
	return roleAccessor()
}

func getOutputFmt() string {
	return outputFmtAccessor()
}

// These accessor variables are set by the root package during init
var (
	kernelAddrAccessor = func() string { return "http://localhost:18400" }
	roleAccessor       = func() string { return "external_agent" }
	outputFmtAccessor  = func() string { return "table" }
)

// SetAccessors is called by the root package to wire up global config accessors.
func SetAccessors(addr, role, output func() string) {
	kernelAddrAccessor = addr
	roleAccessor = role
	outputFmtAccessor = output
}
