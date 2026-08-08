//go:build !windows

package application

func registerConsoleCloseHandler(_ func()) (func(), error) {
	return func() {}, nil
}
