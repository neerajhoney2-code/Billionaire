package network

import "github.com/neerajhoney2-code/billionaire/core"

// MsgType identifies the kind of P2P message.
type MsgType string

const (
	MsgNewBlock   MsgType = "new_block"
	MsgNewTx      MsgType = "new_tx"
	MsgGetChain   MsgType = "get_chain"
	MsgChain      MsgType = "chain"
	MsgGetPeers   MsgType = "get_peers"
	MsgPeers      MsgType = "peers"
)

// Message is the envelope used for all P2P communication.
type Message struct {
	Type    MsgType     `json:"type"`
	Payload interface{} `json:"payload"`
}

// BlockMsg wraps a single block for gossip.
type BlockMsg struct {
	Block *core.Block `json:"block"`
}

// TxMsg wraps a single transaction for gossip.
type TxMsg struct {
	Tx *core.Transaction `json:"tx"`
}

// ChainMsg carries the full chain for sync.
type ChainMsg struct {
	Chain []*core.Block `json:"chain"`
}

// PeersMsg carries a list of peer addresses.
type PeersMsg struct {
	Peers []string `json:"peers"`
}
