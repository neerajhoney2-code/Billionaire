package network

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/neerajhoney2-code/billionaire/core"
)

// Node is the P2P network node.
type Node struct {
	addr       string // "host:port"
	blockchain *core.Blockchain

	mu    sync.RWMutex
	peers map[string]net.Conn // addr → connection

	// NewBlockCh receives blocks to gossip externally (e.g. from ForgeBlock).
	NewBlockCh chan *core.Block
}

// NewNode creates a P2P node bound to addr.
func NewNode(addr string, bc *core.Blockchain) *Node {
	return &Node{
		addr:       addr,
		blockchain: bc,
		peers:      make(map[string]net.Conn),
		NewBlockCh: make(chan *core.Block, 16),
	}
}

// Start listens for incoming connections.
func (n *Node) Start() error {
	ln, err := net.Listen("tcp", n.addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", n.addr, err)
	}
	log.Printf("[p2p] listening on %s", n.addr)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Printf("[p2p] accept error: %v", err)
				continue
			}
			go n.handleConn(conn)
		}
	}()

	// Gossip loop: broadcast newly forged blocks.
	go func() {
		for block := range n.NewBlockCh {
			n.Broadcast(Message{Type: MsgNewBlock, Payload: BlockMsg{Block: block}})
		}
	}()
	return nil
}

// Connect dials a peer and performs chain sync.
func (n *Node) Connect(peerAddr string) {
	if peerAddr == n.addr {
		return
	}
	n.mu.RLock()
	_, already := n.peers[peerAddr]
	n.mu.RUnlock()
	if already {
		return
	}

	conn, err := net.DialTimeout("tcp", peerAddr, 5*time.Second)
	if err != nil {
		log.Printf("[p2p] cannot connect to %s: %v", peerAddr, err)
		return
	}
	n.addPeer(peerAddr, conn)
	log.Printf("[p2p] connected to %s", peerAddr)

	// Request the peer's chain.
	n.sendTo(conn, Message{Type: MsgGetChain})
	// Share our peer list.
	n.sendTo(conn, Message{Type: MsgGetPeers})

	go n.handleConn(conn)
}

// BroadcastTx sends a transaction to all peers.
func (n *Node) BroadcastTx(tx *core.Transaction) {
	n.Broadcast(Message{Type: MsgNewTx, Payload: TxMsg{Tx: tx}})
}

// Broadcast sends a message to all connected peers.
func (n *Node) Broadcast(msg Message) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	for addr, conn := range n.peers {
		if err := n.sendTo(conn, msg); err != nil {
			log.Printf("[p2p] broadcast to %s failed: %v", addr, err)
		}
	}
}

// Peers returns the list of connected peer addresses.
func (n *Node) Peers() []string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	out := make([]string, 0, len(n.peers))
	for addr := range n.peers {
		out = append(out, addr)
	}
	return out
}

// ---- internal ----

func (n *Node) addPeer(addr string, conn net.Conn) {
	n.mu.Lock()
	n.peers[addr] = conn
	n.mu.Unlock()
}

func (n *Node) removePeer(addr string) {
	n.mu.Lock()
	delete(n.peers, addr)
	n.mu.Unlock()
}

func (n *Node) sendTo(conn net.Conn, msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(conn, "%s\n", data)
	return err
}

func (n *Node) handleConn(conn net.Conn) {
	peerAddr := conn.RemoteAddr().String()
	defer func() {
		conn.Close()
		n.removePeer(peerAddr)
		log.Printf("[p2p] disconnected: %s", peerAddr)
	}()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1 MB max message

	for scanner.Scan() {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(scanner.Bytes(), &raw); err != nil {
			log.Printf("[p2p] parse error from %s: %v", peerAddr, err)
			continue
		}
		var msgType MsgType
		if err := json.Unmarshal(raw["type"], &msgType); err != nil {
			continue
		}
		n.handleMessage(conn, msgType, raw["payload"])
	}
}

func (n *Node) handleMessage(conn net.Conn, msgType MsgType, payload json.RawMessage) {
	switch msgType {
	case MsgNewBlock:
		var msg BlockMsg
		if err := json.Unmarshal(payload, &msg); err != nil || msg.Block == nil {
			return
		}
		if err := n.blockchain.AppendBlock(msg.Block); err != nil {
			log.Printf("[p2p] reject block %d: %v", msg.Block.Index, err)
		} else {
			log.Printf("[p2p] accepted block %d from peer", msg.Block.Index)
			// Re-broadcast to other peers.
			n.Broadcast(Message{Type: MsgNewBlock, Payload: msg})
		}

	case MsgNewTx:
		var msg TxMsg
		if err := json.Unmarshal(payload, &msg); err != nil || msg.Tx == nil {
			return
		}
		if err := n.blockchain.SubmitTransaction(msg.Tx); err != nil {
			log.Printf("[p2p] reject tx: %v", err)
		}

	case MsgGetChain:
		chain := n.blockchain.Chain()
		_ = n.sendTo(conn, Message{Type: MsgChain, Payload: ChainMsg{Chain: chain}})

	case MsgChain:
		var msg ChainMsg
		if err := json.Unmarshal(payload, &msg); err != nil {
			return
		}
		if err := n.blockchain.ReplaceChain(msg.Chain); err != nil {
			log.Printf("[p2p] chain replace: %v", err)
		} else {
			log.Printf("[p2p] chain synced: height=%d", n.blockchain.Height())
		}

	case MsgGetPeers:
		peers := n.Peers()
		_ = n.sendTo(conn, Message{Type: MsgPeers, Payload: PeersMsg{Peers: peers}})

	case MsgPeers:
		var msg PeersMsg
		if err := json.Unmarshal(payload, &msg); err != nil {
			return
		}
		for _, peer := range msg.Peers {
			go n.Connect(peer)
		}
	}
}
