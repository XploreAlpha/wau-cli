package stack

import "os"

// osWriteFileImpl is the actual os.WriteFile call,isolated here to allow
// test files to reference os without polluting the package-level imports
// in stack_test.go (which intentionally uses package stack not package stack_test).
func osWriteFileImpl(path string, data []byte, mode uint32) error {
	return os.WriteFile(path, data, os.FileMode(mode))
}
