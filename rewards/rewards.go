package rewards

import "github.com/neerajhoney2-code/billionaire/config"

// BlockReward returns the token reward for producing block at the given height.
// The reward halves every HalvingInterval blocks, with a floor of 1.
func BlockReward(height uint64) uint64 {
	halvings := height / config.HalvingInterval
	reward := config.BlockReward
	for i := uint64(0); i < halvings; i++ {
		reward /= 2
		if reward == 0 {
			return 1
		}
	}
	return reward
}
