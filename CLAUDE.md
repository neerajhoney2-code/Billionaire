# Billionaire — PoS Blockchain + Medicine QR Verification

## Project Overview

A Proof-of-Stake blockchain written in Go with an integrated pharmaceutical supply chain verification system. Every medicine batch gets a QR code linked to a blockchain record. Patients and pharmacists scan the QR with any mobile browser and verify authenticity in under 2 seconds.

## Architecture

```
billionaire/
├── main.go                  Entry point — CLI flags
├── config/config.go         Node configuration + constants
├── crypto/
│   ├── wallet.go            ECDSA P-256 key generation, address derivation
│   └── signing.go           Sign / verify bytes, hash helpers
├── core/
│   ├── transaction.go       Transaction types: transfer, stake, pharma ops
│   ├── block.go             Block struct + SHA-256 hashing
│   ├── state.go             Account state (balance, nonce, stake)
│   ├── genesis.go           Genesis block with initial token allocation
│   └── blockchain.go        Main orchestrator: forge, append, validate, sync
├── consensus/
│   ├── pos.go               Weighted-random validator selection (seed = prevHash)
│   ├── validator.go         Validator registry: register, stake, slash
│   └── slashing.go          Double-sign detection tracker
├── rewards/rewards.go       Block reward + halving schedule
├── pharma/
│   ├── medicine.go          Medicine and SupplyStage structs + payloads
│   ├── supply_chain.go      Registry: register, transfer, dispense, recall
│   ├── qr.go                QR code PNG generation (go-qrcode)
│   └── verify.go            QR hash lookup → genuine/counterfeit result
├── contracts/
│   ├── vm.go                Stack-based bytecode VM (PUSH/POP/ADD/SUB/STORE/LOAD)
│   └── contract.go          Contract deploy + call registry
├── storage/db.go            File-backed key-value store (JSON per namespace)
├── network/
│   ├── node.go              P2P TCP node: connect, gossip, broadcast
│   └── messages.go          Protocol message types
├── api/
│   ├── handlers.go          All REST + pharma + wallet + contract endpoints
│   └── websocket.go         WebSocket hub for live dashboard updates
└── ui/static/
    ├── index.html           Staking dashboard + medicine tracker
    ├── scanner.html         Mobile QR scanner (WebRTC + jsQR, no app needed)
    ├── app.js               Dashboard JS: WebSocket, fetch, render
    └── style.css            Dark theme UI styles
```

## Running the Node

```bash
# Build
go build -o billionaire .

# Single node
./billionaire --port 3000 --api-port 8080

# Multi-node (3 nodes)
./billionaire --port 3000 --api-port 8080
./billionaire --port 3001 --api-port 8081 --peers localhost:3000
./billionaire --port 3002 --api-port 8082 --peers localhost:3000

# Open dashboard
open http://localhost:8080

# Open QR scanner (works on mobile too)
open http://localhost:8080/scanner.html
```

## Key API Endpoints

### Blockchain
| Method | Path | Description |
|--------|------|-------------|
| GET | `/blocks` | Recent 20 blocks |
| GET | `/blocks/:index` | Block by index |
| POST | `/tx` | Submit transaction |
| GET | `/balance/:addr` | Account balance + stake |
| GET | `/validators` | Active validators + stakes |
| POST | `/stake` | Stake tokens |
| POST | `/unstake` | Unstake tokens |
| POST | `/mine` | Forge a new block |
| GET | `/chain/validate` | Full chain integrity check |
| GET | `/ws` | WebSocket live stream |

### Medicine / Pharma
| Method | Path | Description |
|--------|------|-------------|
| POST | `/pharma/register` | Register batch → returns QR PNG |
| GET | `/pharma/:batchID` | Get medicine details |
| POST | `/pharma/transfer` | Transfer custody |
| POST | `/pharma/dispense` | Dispense to patient |
| POST | `/pharma/recall` | Recall a batch |
| GET | `/pharma/list` | All registered medicines |
| GET | `/verify/:qrHash` | **QR scan verification endpoint** |

### Wallet & Contracts
| Method | Path | Description |
|--------|------|-------------|
| POST | `/wallet/new` | Generate keypair (private key shown once) |
| POST | `/contract/deploy` | Deploy bytecode contract |
| GET | `/contract/:addr` | Get contract + storage |

## PoS Consensus

1. Validators stake tokens via `POST /stake` (minimum 100 tokens).
2. On `POST /mine`, the node calls `validators.SelectValidator(prevHash)`:
   - Derives a deterministic seed from the previous block hash.
   - Performs weighted-random selection: probability = `stake / totalStake`.
3. The selected validator's address is recorded in the block.
4. If the block hash is wrong → validator is slashed (10% stake burned).
5. Double-sign (two different hashes at same height) → slash triggered.
6. Block reward: 10 tokens, halves every 1,000,000 blocks.

## Medicine QR Flow

```
Manufacturer calls POST /pharma/register
  → TxRegisterMed added to mempool
  → POST /mine confirms the block
  → QR hash = SHA-256(batchID + manufacturer + manufDate)
  → QR PNG returned encoding http://host/verify/<qrHash>

Patient scans QR with scanner.html
  → jsQR decodes URL client-side (<1s)
  → Fetch GET /verify/<qrHash>
  → Response: genuine=true/false + full supply chain history
  → Total time: <2 seconds
```

## Transaction Types

| Type | Description |
|------|-------------|
| `transfer` | Token transfer between addresses |
| `stake` | Lock tokens to become a validator |
| `unstake` | Release staked tokens |
| `register_medicine` | Register a new drug batch |
| `transfer_medicine` | Transfer custody (distributor/retailer) |
| `dispense_medicine` | Final hand-off to patient |
| `recall_medicine` | Mark batch as recalled |
| `deploy_contract` | Deploy smart contract bytecode |
| `call_contract` | Execute a deployed contract |
| `coinbase` | Genesis allocation (no signature required) |

## Smart Contract VM Opcodes

| Opcode | Byte | Description |
|--------|------|-------------|
| PUSH | `0x01` | Push 8-byte uint64 onto stack |
| POP | `0x02` | Discard top of stack |
| ADD | `0x03` | Pop a, b → push a+b |
| SUB | `0x04` | Pop a, b → push a-b |
| MUL | `0x05` | Pop a, b → push a*b |
| DIV | `0x06` | Pop a, b → push a/b |
| STORE | `0x07` | Pop value, write to key in contract storage |
| LOAD | `0x08` | Read key from contract storage, push value |
| HALT | `0xFF` | Stop execution |

## Configuration Constants (`config/config.go`)

| Constant | Value | Description |
|----------|-------|-------------|
| `BlockReward` | 10 | Tokens per block |
| `MinStake` | 100 | Minimum tokens to register as validator |
| `SlashPercent` | 0.10 | Fraction of stake burned on violation |
| `MaxTxPerBlock` | 100 | Max transactions per block |
| `HalvingInterval` | 1,000,000 | Blocks between reward halvings |

## Dependencies

```
github.com/gorilla/websocket   v1.5.3   WebSocket for live UI updates
github.com/skip2/go-qrcode    v0.0.0   QR code PNG generation
```

All cryptography uses Go standard library (`crypto/ecdsa`, `crypto/sha256`, `elliptic.P256`).

## BNB Chain / BEP-20 Token Integration (Planned)

The user owns a self-made BEP-20 token on BNB Chain (Binance Smart Chain).
Three integration options are planned — awaiting token contract address and use-case confirmation:

### Option 1 — Balance Display (Easiest)
Query BSC via public RPC and show the user's BEP-20 token balance directly
on the Billionaire dashboard alongside the native chain balance.

- No new backend needed — pure frontend `eth_call` to BSC RPC
- Files to add: `ui/static/bnb.js` — reads balance via BSC JSON-RPC
- Files to edit: `ui/static/index.html` — add BNB token balance row to Node Info card

### Option 2 — Token Bridge (Medium)
Users lock BEP-20 tokens on BSC → equivalent tokens minted on Billionaire chain.
Locked tokens can then be used for staking and medicine registration fees.

```
BNB Chain                        Billionaire Chain
[BEP-20 Token Contract]  ──→──  [Billionaire Token]
Lock N tokens            bridge  Credit N tokens
                                 → Stake → Validate blocks
```

- New file: `bnb/bridge.go` — watches BSC lock events via RPC polling
- New file: `bnb/client.go` — BSC JSON-RPC client (eth_call, eth_getLogs)
- Edit: `core/transaction.go` — add `TxBridgeIn` / `TxBridgeOut` tx types
- Edit: `api/handlers.go` — add `POST /bridge/deposit`, `POST /bridge/withdraw`

### Option 3 — BEP-20 as Staking Token (Advanced)
Billionaire chain validator eligibility is gated by BEP-20 token ownership.
Only wallets holding ≥ MinStake of the BEP-20 token on BSC can register as validators.

- New file: `bnb/oracle.go` — polls BSC every N blocks, caches token balances
- Edit: `consensus/validator.go` — check oracle balance before `Register()`
- Edit: `config/config.go` — add `BNBTokenAddress`, `BSCRpcURL` constants

### BSC Configuration (fill in when ready)
```go
// config/config.go
BNBTokenAddress = "0x..."           // Your BEP-20 contract address
BSCRpcURL       = "https://bsc-dataseed.binance.org"
BSCChainID      = 56
```

### Required Information from User
- BEP-20 token **contract address** (`0x...`)
- Token **name and symbol**
- Chosen **integration option** (1, 2, or 3)
- Whether medicine registration fees should be paid in BEP-20 or native token

## Development Branch

Active development: `claude/build-pos-blockchain-7kEJz`
