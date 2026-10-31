package bridge

// Os testes não tocam o chaveiro real da sessão.
func init() { SetDeviceKeeper(nil) }
