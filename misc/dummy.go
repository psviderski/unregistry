// This is a dummy Go file to be built as part of the goreleaser.
// It is a workaround for the GoReleaser having difficulties with publishing
// homebrew casks without an actual built binary.
// Why not building a real unregistry binary? We want to release homebrew casks
// for multiple architectures and OSes, so building a real binary
// would be slow due to cross-compilation.
package main

func main() {}
