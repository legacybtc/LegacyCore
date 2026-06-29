package version

const (
	WalletName    = "Legacy Wallet"
	WalletVersion = "1.0.12"
	CoreName      = "Legacy Core"
	CoreVersion   = "1.0.12"
)

var (
	CoreCommit  = "unknown"
	BuildTime   = "unknown"
)

func WalletFull() string {
	return WalletName + " " + WalletVersion
}

func CoreFull() string {
	return CoreName + " " + CoreVersion
}

func BuildInfo() map[string]string {
	return map[string]string{
		"version": CoreVersion,
		"commit":  CoreCommit,
		"built":   BuildTime,
	}
}
