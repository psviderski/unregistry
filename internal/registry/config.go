package registry

// Config represents the registry configuration.
type Config struct {
	// Addr is the address on which the registry server will listen.
	Addr string
	// ContainerdSock is the path to the containerd.sock socket.
	ContainerdSock string
	// ContainerdNamespace is the containerd namespace to use.
	ContainerdNamespace string
	LogLevel            string
	//HTTPSecret string
}
