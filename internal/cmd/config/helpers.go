package config

// Accessor functions for global config

func getKernelAddr() string { return kernelAddrAccessor() }
func getOutputFmt() string  { return outputFmtAccessor() }

var (
	kernelAddrAccessor = func() string { return "http://localhost:18400" }
	outputFmtAccessor  = func() string { return "table" }
)

// SetAccessors is called by the root package to wire up global config accessors.
func SetAccessors(addr, output func() string) {
	kernelAddrAccessor = addr
	outputFmtAccessor = output
}
