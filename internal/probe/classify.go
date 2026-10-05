package probe

// classifyPort maps a TCP port number to its pinned service label.
func classifyPort(port int) string {
	switch port {
	case 80, 443, 8080, 8443:
		return "Web"
	case 445:
		return "SMB"
	case 3389:
		return "RDP"
	case 22:
		return "SSH"
	case 21:
		return "FTP"
	case 515, 631, 9100:
		return "Printer"
	default:
		return "unknown"
	}
}
