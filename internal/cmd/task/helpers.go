package task

// Accessor functions for global config

func getKernelAddr() string { return kernelAddrAccessor() }
func getRole() string       { return roleAccessor() }
func getOutputFmt() string  { return outputFmtAccessor() }

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
