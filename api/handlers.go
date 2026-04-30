package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/neerajhoney2-code/billionaire/contracts"
	"github.com/neerajhoney2-code/billionaire/core"
	"github.com/neerajhoney2-code/billionaire/crypto"
	"github.com/neerajhoney2-code/billionaire/network"
	"github.com/neerajhoney2-code/billionaire/pharma"
)

// Server holds all dependencies for the HTTP handlers.
type Server struct {
	bc        *core.Blockchain
	node      *network.Node
	contracts *contracts.Registry
	hub       *Hub
	host      string // used for QR code URL generation
}

func NewServer(bc *core.Blockchain, node *network.Node, cr *contracts.Registry, host string) *Server {
	return &Server{bc: bc, node: node, contracts: cr, hub: NewHub(), host: host}
}

// RegisterRoutes attaches all HTTP routes to mux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Blockchain
	mux.HandleFunc("/blocks", s.handleBlocks)
	mux.HandleFunc("/blocks/", s.handleBlockByIndex)
	mux.HandleFunc("/txpool", s.handleTxPool)
	mux.HandleFunc("/tx", s.handleSubmitTx)
	mux.HandleFunc("/tx/", s.handleGetTx)
	mux.HandleFunc("/balance/", s.handleBalance)
	mux.HandleFunc("/validators", s.handleValidators)
	mux.HandleFunc("/stake", s.handleStake)
	mux.HandleFunc("/unstake", s.handleUnstake)
	mux.HandleFunc("/mine", s.handleMine)
	mux.HandleFunc("/peers", s.handlePeers)
	mux.HandleFunc("/chain/validate", s.handleValidateChain)
	mux.HandleFunc("/ws", s.hub.ServeWS)

	// Pharma
	mux.HandleFunc("/pharma/register", s.handlePharmaRegister)
	mux.HandleFunc("/pharma/transfer", s.handlePharmaTransfer)
	mux.HandleFunc("/pharma/dispense", s.handlePharmaDispense)
	mux.HandleFunc("/pharma/recall", s.handlePharmaRecall)
	mux.HandleFunc("/pharma/list", s.handlePharmaList)
	mux.HandleFunc("/pharma/", s.handlePharmaGet)
	mux.HandleFunc("/verify/", s.handleVerify)

	// Wallet
	mux.HandleFunc("/wallet/new", s.handleNewWallet)

	// Contracts
	mux.HandleFunc("/contract/deploy", s.handleContractDeploy)
	mux.HandleFunc("/contract/", s.handleContractGet)

	// UI static files
	mux.Handle("/", http.FileServer(http.Dir("./ui/static")))
}

// ---- helpers ----

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func errJSON(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeBody(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// ---- Blockchain handlers ----

func (s *Server) handleBlocks(w http.ResponseWriter, r *http.Request) {
	blocks := s.bc.RecentBlocks(20)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"blocks": blocks,
		"height": s.bc.Height(),
	})
}

func (s *Server) handleBlockByIndex(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		errJSON(w, http.StatusBadRequest, "missing index")
		return
	}
	idx, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		errJSON(w, http.StatusBadRequest, "invalid index")
		return
	}
	block, err := s.bc.GetBlock(idx)
	if err != nil {
		errJSON(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, block)
}

func (s *Server) handleTxPool(w http.ResponseWriter, r *http.Request) {
	txs := s.bc.Mempool().All()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"transactions": txs,
		"count":        len(txs),
	})
}

func (s *Server) handleSubmitTx(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errJSON(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var tx core.Transaction
	if err := decodeBody(r, &tx); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := s.bc.SubmitTransaction(&tx); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.node != nil {
		s.node.BroadcastTx(&tx)
	}
	writeJSON(w, http.StatusCreated, map[string]string{"tx_id": tx.ID, "status": "pending"})
}

func (s *Server) handleGetTx(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		errJSON(w, http.StatusBadRequest, "missing tx id")
		return
	}
	txID := parts[2]
	tx, blockIdx, ok := s.bc.FindTransaction(txID)
	if !ok {
		errJSON(w, http.StatusNotFound, "transaction not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"transaction": tx,
		"block_index": blockIdx,
	})
}

func (s *Server) handleBalance(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		errJSON(w, http.StatusBadRequest, "missing address")
		return
	}
	addr := parts[2]
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"address":       addr,
		"balance":       s.bc.State().Balance(addr),
		"total_balance": s.bc.State().TotalBalance(addr),
		"stake":         s.bc.State().Stake(addr),
	})
}

func (s *Server) handleValidators(w http.ResponseWriter, r *http.Request) {
	vs := s.bc.Validators().All()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"validators":  vs,
		"count":       len(vs),
		"total_stake": s.bc.Validators().TotalStake(),
	})
}

func (s *Server) handleStake(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errJSON(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req struct {
		Address   string `json:"address"`
		PublicKey string `json:"public_key"`
		Amount    uint64 `json:"amount"`
	}
	if err := decodeBody(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.bc.StakeTokens(req.Address, req.PublicKey, req.Amount); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	v, _ := s.bc.Validators().Get(req.Address)
	writeJSON(w, http.StatusOK, map[string]interface{}{"validator": v, "message": "staked successfully"})
}

func (s *Server) handleUnstake(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errJSON(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req struct {
		Address string `json:"address"`
		Amount  uint64 `json:"amount"`
	}
	if err := decodeBody(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.bc.UnstakeTokens(req.Address, req.Amount); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": fmt.Sprintf("unstaked %d tokens", req.Amount)})
}

func (s *Server) handleMine(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errJSON(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	block, err := s.bc.ForgeBlock()
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	// Broadcast to peers.
	if s.node != nil {
		s.node.NewBlockCh <- block
	}
	// Push live event to WebSocket clients.
	s.hub.Broadcast(map[string]interface{}{"event": "new_block", "block": block})
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"block":   block,
		"message": fmt.Sprintf("block %d forged by %s", block.Index, block.Validator),
	})
}

func (s *Server) handlePeers(w http.ResponseWriter, r *http.Request) {
	var peers []string
	if s.node != nil {
		peers = s.node.Peers()
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"peers": peers})
}

func (s *Server) handleValidateChain(w http.ResponseWriter, r *http.Request) {
	ok, msg := s.bc.ValidateChain()
	writeJSON(w, http.StatusOK, map[string]interface{}{"valid": ok, "message": msg})
}

// ---- Pharma handlers ----

func (s *Server) handlePharmaRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errJSON(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req struct {
		pharma.RegisterPayload
		Manufacturer string `json:"manufacturer"`
		PublicKey    string `json:"public_key"`
		PrivateKey   string `json:"private_key"` // to sign the tx
		Nonce        uint64 `json:"nonce"`
		ReturnQR     bool   `json:"return_qr"` // if true, respond with PNG
	}
	if err := decodeBody(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	// Build and sign the pharma tx.
	tx, err := core.NewPharmaTx(core.TxRegisterMed, req.Manufacturer, req.RegisterPayload, 1, req.Nonce, req.PublicKey)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if req.PrivateKey != "" {
		wallet, err := crypto.FromPrivateKeyHex(req.PrivateKey)
		if err != nil {
			errJSON(w, http.StatusBadRequest, "invalid private key: "+err.Error())
			return
		}
		sig, err := wallet.Sign([]byte(fmt.Sprintf("%s|%s|%s|%s|%d|%d|%d|%x",
			tx.Type, tx.Sender, tx.Recipient, tx.ID, tx.Amount, tx.Fee, tx.Nonce, tx.Data)))
		if err != nil {
			errJSON(w, http.StatusInternalServerError, err.Error())
			return
		}
		tx.Signature = sig
	}

	if err := s.bc.SubmitTransaction(tx); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	// Mine immediately so the medicine is confirmed.
	block, err := s.bc.ForgeBlock()
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "block forge: "+err.Error())
		return
	}

	med, ok := s.bc.PharmaRegistry().GetByBatchID(req.BatchID)
	if !ok {
		errJSON(w, http.StatusInternalServerError, "medicine not found after forging")
		return
	}

	if req.ReturnQR {
		var buf bytes.Buffer
		if err := pharma.GenerateQRPNG(med.QRHash, s.host, &buf); err != nil {
			errJSON(w, http.StatusInternalServerError, "qr generation: "+err.Error())
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("X-QR-Hash", med.QRHash)
		w.Header().Set("X-Block-Index", strconv.FormatUint(block.Index, 10))
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(buf.Bytes())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"medicine":   med,
		"qr_hash":    med.QRHash,
		"verify_url": pharma.VerifyURL(med.QRHash, s.host),
		"block":      block.Index,
		"tx_id":      tx.ID,
	})
}

func (s *Server) handlePharmaTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errJSON(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req struct {
		pharma.TransferPayload
		Sender    string `json:"sender"`
		PublicKey string `json:"public_key"`
		Nonce     uint64 `json:"nonce"`
	}
	if err := decodeBody(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	tx, err := core.NewPharmaTx(core.TxTransferMed, req.Sender, req.TransferPayload, 1, req.Nonce, req.PublicKey)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.bc.SubmitTransaction(tx); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"tx_id": tx.ID, "status": "pending"})
}

func (s *Server) handlePharmaDispense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errJSON(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req struct {
		pharma.DispensePayload
		Sender    string `json:"sender"`
		PublicKey string `json:"public_key"`
		Nonce     uint64 `json:"nonce"`
	}
	if err := decodeBody(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	tx, err := core.NewPharmaTx(core.TxDispenseMed, req.Sender, req.DispensePayload, 1, req.Nonce, req.PublicKey)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.bc.SubmitTransaction(tx); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"tx_id": tx.ID, "status": "pending"})
}

func (s *Server) handlePharmaRecall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errJSON(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req struct {
		pharma.RecallPayload
		Sender    string `json:"sender"`
		PublicKey string `json:"public_key"`
		Nonce     uint64 `json:"nonce"`
	}
	if err := decodeBody(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	tx, err := core.NewPharmaTx(core.TxRecallMed, req.Sender, req.RecallPayload, 1, req.Nonce, req.PublicKey)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.bc.SubmitTransaction(tx); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"tx_id": tx.ID, "status": "pending"})
}

func (s *Server) handlePharmaList(w http.ResponseWriter, r *http.Request) {
	meds := s.bc.PharmaRegistry().All()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"medicines": meds,
		"count":     len(meds),
	})
}

func (s *Server) handlePharmaGet(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		errJSON(w, http.StatusBadRequest, "missing batch id")
		return
	}
	batchID := parts[2]
	med, ok := s.bc.PharmaRegistry().GetByBatchID(batchID)
	if !ok {
		errJSON(w, http.StatusNotFound, "medicine not found")
		return
	}
	writeJSON(w, http.StatusOK, med)
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		errJSON(w, http.StatusBadRequest, "missing qr hash")
		return
	}
	qrHash := parts[2]
	result := s.bc.PharmaRegistry().Verify(qrHash)
	status := http.StatusOK
	if !result.Genuine {
		status = http.StatusOK // still 200 — frontend reads the Genuine field
	}
	writeJSON(w, status, result)
}

// ---- Wallet handler ----

func (s *Server) handleNewWallet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errJSON(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	wallet, err := crypto.Generate()
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	privHex, err := wallet.PrivateKeyHex()
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"address":     wallet.Address,
		"public_key":  wallet.PublicKey,
		"private_key": privHex,
		"warning":     "Store the private key securely. It is never stored server-side.",
	})
}

// ---- Contract handlers ----

func (s *Server) handleContractDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errJSON(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req struct {
		Deployer string `json:"deployer"`
		Code     []byte `json:"code"` // raw bytecode bytes (base64 by JSON encoding)
	}
	if err := decodeBody(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	c, err := s.contracts.Deploy(req.Deployer, req.Code)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"address":  c.Address,
		"deployer": c.Deployer,
	})
}

func (s *Server) handleContractGet(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		errJSON(w, http.StatusBadRequest, "missing address")
		return
	}
	addr := parts[2]
	c, ok := s.contracts.Get(addr)
	if !ok {
		errJSON(w, http.StatusNotFound, "contract not found")
		return
	}
	writeJSON(w, http.StatusOK, c)
}
