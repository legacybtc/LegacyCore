package version

const (
	WalletName    = "Legacy Wallet"
	WalletVersion = "1.0.3"
	CoreName      = "Legacy Core"
	CoreVersion   = "1.0.3"
)

func WalletFull() string {
	return WalletName + " " + WalletVersion
}

func CoreFull() string {
	return CoreName + " " + CoreVersion
}
