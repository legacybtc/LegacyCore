package version

const (
	WalletName    = "Legacy Wallet"
	WalletVersion = "1.0.5"
	CoreName      = "Legacy Core"
	CoreVersion   = "1.0.5"
)

func WalletFull() string {
	return WalletName + " " + WalletVersion
}

func CoreFull() string {
	return CoreName + " " + CoreVersion
}
