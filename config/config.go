package config

// Config holds all node configuration.
type Config struct {
	P2PPort    int
	APIPort    int
	Peers      []string
	DataDir    string
	NodeID     string
	PrivateKey string // hex-encoded private key for this node's validator identity
}

const (
	BlockReward    uint64 = 10    // tokens rewarded per block
	MinStake       uint64 = 100   // minimum tokens to stake as validator
	SlashPercent          = 0.10  // fraction of stake burned on violation
	MaxTxPerBlock         = 100
	HalvingInterval       = 1_000_000 // blocks between reward halvings
)

func Default() *Config {
	return &Config{
		P2PPort: 3000,
		APIPort: 8080,
		DataDir: "./data",
	}
}
