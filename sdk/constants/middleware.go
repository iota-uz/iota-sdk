package constants

type SdkContextKey string

const (
	TxKey     SdkContextKey = "tx"
	ParamsKey SdkContextKey = "params"
	LoggerKey SdkContextKey = "logger"
)
