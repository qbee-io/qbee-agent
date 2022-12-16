package inventory

type Ports struct {
	Ports []Port `json:"items"`
}

type Port struct {
	// Protocol - network protocol used (e.g. "tcp", "tcp6", "udp" or "udp6").
	Protocol string `json:"proto"`

	// Socket - which socket is listening (e.g. "0.0.0.0:69").
	Socket string `json:"socket"`

	// Process - which process is controlling the socket (e.g. "/usr/sbin/in.tftpd ...").
	Process string `json:"proc_info"`
}
