'use strict';

const BASE = '';

// ---- WebSocket live updates ----
let ws;
function connectWS() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  ws = new WebSocket(`${proto}://${location.host}/ws`);
  ws.onopen = () => {
    document.getElementById('wsDot').className = 'dot dot-green';
    refresh();
  };
  ws.onmessage = e => {
    const data = JSON.parse(e.data);
    if (data.event === 'new_block') {
      logAction(`Block ${data.block.index} forged by ${data.block.validator.slice(0,12)}…`);
      refresh();
    }
  };
  ws.onclose = () => {
    document.getElementById('wsDot').className = 'dot dot-red';
    setTimeout(connectWS, 3000);
  };
}

function logAction(msg) {
  const log = document.getElementById('actionLog');
  log.textContent = `[${new Date().toLocaleTimeString()}] ${msg}\n` + log.textContent;
}

// ---- Data refresh ----
async function refresh() {
  try {
    const [blocks, validators, meds, txpool] = await Promise.all([
      fetch(BASE + '/blocks').then(r => r.json()),
      fetch(BASE + '/validators').then(r => r.json()),
      fetch(BASE + '/pharma/list').then(r => r.json()),
      fetch(BASE + '/txpool').then(r => r.json()),
    ]);

    document.getElementById('height').textContent = blocks.height ?? '—';
    document.getElementById('pending').textContent = txpool.count ?? 0;
    document.getElementById('validatorCount').textContent = validators.count ?? 0;
    document.getElementById('totalStake').textContent = (validators.total_stake ?? 0).toLocaleString();
    document.getElementById('medCount').textContent = meds.count ?? 0;

    renderValidators(validators.validators || []);
    renderBlocks(blocks.blocks || []);
    renderMeds(meds.medicines || []);
  } catch (e) {
    logAction('Refresh error: ' + e.message);
  }
}

function renderValidators(vs) {
  const tbody = document.querySelector('#validatorsTable tbody');
  tbody.innerHTML = vs.map(v => `
    <tr>
      <td><code>${v.address}</code></td>
      <td>${(v.stake || 0).toLocaleString()}</td>
      <td>${v.blocks_produced ?? 0}</td>
      <td>${v.slash_count ?? 0}</td>
      <td>${v.active ? '<span class="badge badge-green">Active</span>' : '<span class="badge badge-red">Inactive</span>'}</td>
    </tr>`).join('');
}

function renderBlocks(blocks) {
  const tbody = document.querySelector('#blocksTable tbody');
  tbody.innerHTML = [...blocks].reverse().map(b => `
    <tr>
      <td>${b.index}</td>
      <td><code>${(b.validator || '').slice(0, 16)}…</code></td>
      <td>${(b.transactions || []).length}</td>
      <td>${b.reward ?? 0}</td>
      <td>${new Date(b.timestamp / 1e6).toLocaleTimeString()}</td>
    </tr>`).join('');
}

let allMeds = [];
function renderMeds(meds) {
  allMeds = meds;
  filterMeds();
}

function filterMeds() {
  const q = (document.getElementById('medSearch').value || '').toLowerCase();
  const filtered = allMeds.filter(m =>
    (m.batch_id || '').toLowerCase().includes(q) ||
    (m.name || '').toLowerCase().includes(q)
  );
  const statusBadge = s => {
    const map = { manufactured:'badge-blue', distributed:'badge-yellow', retail:'badge-yellow', dispensed:'badge-green', recalled:'badge-red' };
    return `<span class="badge ${map[s] || 'badge-blue'}">${s}</span>`;
  };
  const tbody = document.querySelector('#medsTable tbody');
  tbody.innerHTML = filtered.map(m => `
    <tr>
      <td><code>${m.batch_id}</code></td>
      <td>${m.name}</td>
      <td><code>${(m.manufacturer || '').slice(0,12)}…</code></td>
      <td>${statusBadge(m.status)}</td>
      <td>${m.expiry_date ? new Date(m.expiry_date / 1e6).toLocaleDateString() : '—'}</td>
      <td><a href="/scanner.html#${m.qr_hash}" style="color:#4fc3f7" onclick="openScanner('${m.qr_hash}')">Verify</a></td>
    </tr>`).join('');
}

function openScanner(hash) {
  sessionStorage.setItem('verifyHash', hash);
  window.location.href = '/scanner.html';
}

// ---- Actions ----
async function mine() {
  try {
    const r = await fetch(BASE + '/mine', { method: 'POST' });
    const d = await r.json();
    if (d.error) { logAction('Mine error: ' + d.error); return; }
    logAction(d.message || 'Block mined');
    refresh();
  } catch(e) { logAction('Mine failed: ' + e.message); }
}

async function generateWallet() {
  try {
    const r = await fetch(BASE + '/wallet/new', { method: 'POST' });
    const d = await r.json();
    const msg = `New wallet: ${d.address}`;
    logAction(msg);
    prompt('Address (copy this):', d.address);
    if (confirm('Show private key? Store it securely!')) {
      prompt('Private key (KEEP SECRET):', d.private_key);
    }
  } catch(e) { logAction('Wallet error: ' + e.message); }
}

async function validateChain() {
  try {
    const r = await fetch(BASE + '/chain/validate');
    const d = await r.json();
    logAction(`Chain valid: ${d.valid} — ${d.message}`);
  } catch(e) { logAction('Validate error: ' + e.message); }
}

async function sendTx(e) {
  e.preventDefault();
  const tx = {
    type: 'transfer',
    sender: document.getElementById('txSender').value,
    recipient: document.getElementById('txRecipient').value,
    amount: parseInt(document.getElementById('txAmount').value),
    fee: parseInt(document.getElementById('txFee').value) || 0,
    nonce: parseInt(document.getElementById('txNonce').value) || 0,
    public_key: document.getElementById('txPubKey').value,
    signature: document.getElementById('txSig').value,
    timestamp: Date.now() * 1e6,
  };
  tx.id = await sha256(`${tx.type}|${tx.sender}|${tx.recipient}|${tx.amount}|${tx.fee}|${tx.nonce}|${tx.timestamp}`);
  try {
    const r = await fetch(BASE + '/tx', { method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify(tx) });
    const d = await r.json();
    logAction(d.error ? 'Tx error: '+d.error : 'Tx submitted: '+d.tx_id);
  } catch(e) { logAction('Tx failed: '+e.message); }
}

async function stake(e) {
  e.preventDefault();
  const body = {
    address: document.getElementById('stakeAddr').value,
    public_key: document.getElementById('stakePub').value,
    amount: parseInt(document.getElementById('stakeAmt').value),
  };
  try {
    const r = await fetch(BASE + '/stake', { method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify(body) });
    const d = await r.json();
    logAction(d.error ? 'Stake error: '+d.error : 'Staked successfully');
    refresh();
  } catch(e) { logAction('Stake failed: '+e.message); }
}

async function unstake() {
  const body = {
    address: document.getElementById('stakeAddr').value,
    amount: parseInt(document.getElementById('stakeAmt').value),
  };
  try {
    const r = await fetch(BASE + '/unstake', { method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify(body) });
    const d = await r.json();
    logAction(d.error ? 'Unstake error: '+d.error : 'Unstaked successfully');
    refresh();
  } catch(e) { logAction('Unstake failed: '+e.message); }
}

async function registerMed(e) {
  e.preventDefault();
  const now = Date.now() * 1e6;
  const body = {
    batch_id: document.getElementById('medBatchID').value,
    name: document.getElementById('medName').value,
    manufacturer: document.getElementById('medMfr').value,
    public_key: document.getElementById('medPubKey').value,
    private_key: document.getElementById('medPrivKey').value,
    composition: document.getElementById('medComposition').value,
    quantity: parseInt(document.getElementById('medQty').value),
    location: document.getElementById('medLocation').value,
    manuf_date: now,
    expiry_date: now + 2 * 365 * 24 * 3600 * 1e9, // 2 years
    nonce: 0,
  };
  try {
    const r = await fetch(BASE + '/pharma/register', {
      method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify(body)
    });
    if (r.headers.get('Content-Type') === 'image/png') {
      const blob = await r.blob();
      const url = URL.createObjectURL(blob);
      const qrHash = r.headers.get('X-QR-Hash') || '';
      document.getElementById('qrResult').innerHTML = `
        <img src="${url}" style="width:200px;border:3px solid #4fc3f7;border-radius:8px;margin-bottom:8px"><br>
        <small style="color:#90a4ae">QR Hash: ${qrHash.slice(0,20)}…</small><br>
        <a href="/scanner.html" style="color:#4fc3f7;font-size:0.85rem">→ Open Scanner</a>`;
      logAction('Medicine registered, QR generated');
    } else {
      const d = await r.json();
      if (d.error) { logAction('Register error: '+d.error); return; }
      document.getElementById('qrResult').innerHTML = `
        <small style="color:#69f0ae">Registered! QR Hash: ${(d.qr_hash||'').slice(0,20)}…</small><br>
        <a href="/verify/${d.qr_hash}" style="color:#4fc3f7;font-size:0.85rem">→ Verify URL</a>`;
      logAction('Medicine registered: '+d.qr_hash);
    }
    refresh();
  } catch(err) { logAction('Register failed: '+err.message); }
}

// Simple SHA-256 helper for computing tx ID client-side.
async function sha256(msg) {
  const buf = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(msg));
  return Array.from(new Uint8Array(buf)).map(b => b.toString(16).padStart(2,'0')).join('');
}

// ---- Init ----
connectWS();
setInterval(refresh, 10000);
