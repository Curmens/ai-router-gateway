import React, { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import {
  Link, Key, RefreshCw, CheckCircle, AlertTriangle,
  Settings as SettingsIcon, Plug, Eye, EyeOff
} from 'lucide-react';
import { ProviderLogo } from './ProviderLogos';

const PROVIDER_META = {
  subscription: { label: 'Claude (Anthropic)', color: '#d4a574', type: 'cli', desc: 'Anthropic Claude via CLI subscription' },
  agy:          { label: 'Antigravity (AGY)',  color: '#276ef1', type: 'cli', desc: 'Antigravity agy CLI (Google OAuth)' },
  gemini:       { label: 'Google Gemini',      color: '#4285f4', type: 'api_key', desc: 'Google Gemini API' },
  openai:       { label: 'OpenAI',             color: '#00a67e', type: 'api_key', desc: 'OpenAI GPT API' },
  ollama:       { label: 'Ollama (Local)',      color: '#ffffff', type: 'local', desc: 'Local Ollama inference server' },
};

const MOCK_PROVIDERS = [
  { name: 'subscription', is_healthy: false, enabled: true },
  { name: 'agy', is_healthy: false, enabled: true },
  { name: 'gemini', is_healthy: false, enabled: true },
  { name: 'openai', is_healthy: false, enabled: true },
  { name: 'ollama', is_healthy: false, enabled: true },
];

const fadeUp = {
  initial: { opacity: 0, y: 12 },
  animate: { opacity: 1, y: 0, transition: { duration: 0.35, ease: [0.4, 0, 0.2, 1] } }
};

function ProviderCard({ prov, apiHost, apiKey: adminKey, isOnline }) {
  const meta = PROVIDER_META[prov.name] || {};
  const [value, setValue] = useState('');
  const [showKey, setShowKey] = useState(false);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [status, setStatus] = useState(null);
  const [statusMsg, setStatusMsg] = useState('');
  const [enabled, setEnabled] = useState(prov.enabled !== false);

  const save = async () => {
    if (!isOnline) { setStatus('error'); setStatusMsg('Connect to gateway first'); setTimeout(() => setStatus(null), 3000); return; }
    setSaving(true);
    const body = { enabled };
    if (meta.type === 'api_key' && value) body.api_key = value;
    if (meta.type === 'local' && value) body.base_url = value;
    if (meta.type === 'cli' && value) body.binary_path = value;

    try {
      const res = await fetch(`${apiHost}/v1/admin/providers/${prov.name}`, {
        method: 'PUT',
        headers: { 'Authorization': `Bearer ${adminKey}`, 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      const d = await res.json();
      setStatus(res.ok ? 'ok' : 'error');
      setStatusMsg(res.ok ? 'Saved' : (d.error || 'Save failed'));
    } catch (e) {
      setStatus('error'); setStatusMsg('Network error');
    } finally {
      setSaving(false); setTimeout(() => setStatus(null), 3000);
    }
  };

  const test = async () => {
    if (!isOnline) { setStatus('error'); setStatusMsg('Connect to gateway first'); setTimeout(() => setStatus(null), 3000); return; }
    setTesting(true); setStatus(null);
    try {
      const res = await fetch(`${apiHost}/v1/admin/providers/${prov.name}/test`, {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${adminKey}` },
      });
      const d = await res.json();
      setStatus(d.ok ? 'ok' : 'error');
      setStatusMsg(d.ok ? `Connected — "${d.response?.slice(0, 40)}"` : (d.error?.slice(0, 80) || 'Test failed'));
    } catch (e) {
      setStatus('error'); setStatusMsg('Network error');
    } finally { setTesting(false); }
  };

  return (
    <motion.div variants={fadeUp} className="card-glow">
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <div style={{
            width: 40, height: 40, borderRadius: 10,
            background: `${meta.color}10`,
            display: 'flex', alignItems: 'center', justifyContent: 'center'
          }}>
            <ProviderLogo provider={prov.name} size={24} />
          </div>
          <div>
            <div style={{ fontWeight: 700, fontSize: 15, color: '#fff' }}>{meta.label}</div>
            <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 2 }}>{meta.desc}</div>
          </div>
        </div>
        <label className="toggle">
          <input type="checkbox" checked={enabled} onChange={e => setEnabled(e.target.checked)} />
          <span className="track" /><span className="knob" />
        </label>
      </div>

      {/* Config Input */}
      <div style={{ marginTop: 16, display: 'flex', flexDirection: 'column', gap: 12 }}>
        {meta.type === 'api_key' && (
          <div>
            <label className="label">API Key</label>
            <div style={{ position: 'relative' }}>
              <input
                className="input"
                type={showKey ? 'text' : 'password'}
                placeholder={prov.api_key || 'Enter API key…'}
                value={value}
                onChange={e => setValue(e.target.value)}
                style={{ paddingRight: 40, fontFamily: 'var(--font-mono)', fontSize: 12 }}
              />
              <button
                onClick={() => setShowKey(s => !s)}
                style={{ position: 'absolute', right: 12, top: '50%', transform: 'translateY(-50%)', background: 'none', border: 'none', cursor: 'pointer', color: 'var(--text-muted)', padding: 0 }}
              >
                {showKey ? <EyeOff size={14} /> : <Eye size={14} />}
              </button>
            </div>
          </div>
        )}

        {meta.type === 'local' && (
          <div>
            <label className="label">Server URL</label>
            <input
              className="input"
              type="text"
              placeholder={prov.base_url || 'http://localhost:11434'}
              value={value}
              onChange={e => setValue(e.target.value)}
              style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}
            />
          </div>
        )}

        {meta.type === 'cli' && (
          <div>
            <label className="label">Binary Path</label>
            <input
              className="input"
              type="text"
              placeholder={prov.binary_path || (prov.name === 'subscription' ? '/usr/local/bin/claude' : '/usr/local/bin/agy')}
              value={value}
              onChange={e => setValue(e.target.value)}
              style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}
            />
            <div style={{ fontSize: 11, color: 'var(--text-dim)', marginTop: 6 }}>
              {prov.name === 'subscription'
                ? 'Requires Claude Code CLI installed and authenticated.'
                : 'Requires agy CLI installed and authenticated via Google OAuth.'}
            </div>
          </div>
        )}

        {/* Actions */}
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <button className="btn btn-primary" onClick={save} disabled={saving}>
            {saving && <RefreshCw size={12} className="spin" />}
            Save
          </button>
          <button className="btn btn-ghost" onClick={test} disabled={testing}>
            {testing ? <RefreshCw size={12} className="spin" /> : <Plug size={12} />}
            Test
          </button>
          {status && (
            <span style={{ fontSize: 12, fontWeight: 600, color: status === 'ok' ? 'var(--green)' : 'var(--red)', display: 'flex', alignItems: 'center', gap: 4 }}>
              {status === 'ok' ? <CheckCircle size={12} /> : <AlertTriangle size={12} />}
              {statusMsg}
            </span>
          )}
        </div>
      </div>
    </motion.div>
  );
}

export default function Settings({ settings, setSettings, isOnline }) {
  const [testStatus, setTestStatus] = useState('idle');
  const [testResult, setTestResult] = useState('');
  const [providers, setProviders] = useState([]);

  useEffect(() => {
    if (!isOnline) {
      // Show mock providers when offline so the UI isn't empty
      setProviders(MOCK_PROVIDERS);
      return;
    }
    fetch(`${settings.apiHost}/v1/admin/providers`, {
      headers: { 'Authorization': `Bearer ${settings.apiKey}` }
    })
      .then(r => r.ok ? r.json() : null)
      .then(d => { if (d?.providers) setProviders(d.providers); else setProviders(MOCK_PROVIDERS); })
      .catch(() => setProviders(MOCK_PROVIDERS));
  }, [isOnline, settings.apiHost, settings.apiKey]);

  const verifyConnection = async () => {
    setTestStatus('testing'); setTestResult('');
    try {
      const res = await fetch(`${settings.apiHost}/health`);
      if (res.ok) {
        const d = await res.json();
        setTestStatus('success');
        setTestResult(`Gateway connected — status: ${d.status}, env: ${d.environment}`);
      } else { throw new Error(`Code ${res.status}`); }
    } catch (e) {
      setTestStatus('error');
      setTestResult(`Connection failed. Verify gateway on ${settings.apiHost}`);
    }
  };

  return (
    <motion.div
      initial="initial" animate="animate"
      variants={{ animate: { transition: { staggerChildren: 0.07 } } }}
      style={{ display: 'flex', flexDirection: 'column', gap: 16, maxWidth: 760 }}
    >

      {/* ── Gateway Connection ── */}
      <motion.div variants={fadeUp} className="card">
        <div className="card-header" style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <Link size={12} style={{ color: 'var(--accent)' }} />
          Gateway Connection
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <div>
            <label className="label">API Endpoint</label>
            <input className="input" type="text" value={settings.apiHost}
              onChange={e => setSettings(p => ({ ...p, apiHost: e.target.value }))}
              style={{ fontFamily: 'var(--font-mono)' }} />
          </div>
          <div>
            <label className="label">Authorization Token</label>
            <div style={{ position: 'relative' }}>
              <Key size={14} style={{ position: 'absolute', left: 14, top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
              <input className="input" type="password" value={settings.apiKey}
                onChange={e => setSettings(p => ({ ...p, apiKey: e.target.value }))}
                style={{ paddingLeft: 38, fontFamily: 'var(--font-mono)' }} />
            </div>
          </div>
          <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
            <button className="btn btn-primary" onClick={verifyConnection} disabled={testStatus === 'testing'}>
              {testStatus === 'testing' && <RefreshCw size={13} className="spin" />}
              Verify Connection
            </button>
            <span style={{ fontSize: 13, fontWeight: 700, color: isOnline ? 'var(--green)' : 'var(--red)' }}>
              ● {isOnline ? 'Online' : 'Offline'}
            </span>
          </div>

          {testStatus === 'success' && (
            <div className="embed" style={{ borderLeftColor: 'var(--green)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, color: 'var(--green)', fontSize: 13 }}>
                <CheckCircle size={14} />{testResult}
              </div>
            </div>
          )}
          {testStatus === 'error' && (
            <div className="embed" style={{ borderLeftColor: 'var(--red)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, color: 'var(--red)', fontSize: 13 }}>
                <AlertTriangle size={14} />{testResult}
              </div>
            </div>
          )}
        </div>
      </motion.div>

      {/* ── Provider Connectors ── */}
      <motion.div variants={fadeUp} style={{ display: 'flex', alignItems: 'center', gap: 8, paddingLeft: 4 }}>
        <Plug size={13} style={{ color: 'var(--accent)' }} />
        <span style={{ fontSize: 11, fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
          Provider Connectors {!isOnline && '(Offline Preview)'}
        </span>
      </motion.div>

      {providers.map(p => (
        <ProviderCard key={p.name} prov={p} apiHost={settings.apiHost} apiKey={settings.apiKey} isOnline={isOnline} />
      ))}

      {/* ── Dashboard Config ── */}
      <motion.div variants={fadeUp} className="card">
        <div className="card-header" style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <SettingsIcon size={12} style={{ color: 'var(--purple)' }} />
          Dashboard Configuration
        </div>
        <div style={{ display: 'flex', flexDirection: 'column' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '14px 0' }}>
            <div>
              <div style={{ fontSize: 14, fontWeight: 600, color: '#fff' }}>Console Polling Speed</div>
              <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 2 }}>How frequently telemetry is refreshed.</div>
            </div>
            <select className="select" value={settings.pollInterval}
              onChange={e => setSettings(p => ({ ...p, pollInterval: parseInt(e.target.value, 10) }))}>
              <option value="1000">Fast (1.0s)</option>
              <option value="2000">Normal (2.0s)</option>
              <option value="5000">Slow (5.0s)</option>
            </select>
          </div>
          <div className="divider" />
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '14px 0' }}>
            <div>
              <div style={{ fontSize: 14, fontWeight: 600, color: '#fff' }}>Offline Simulation</div>
              <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 2 }}>Enable mock data when gateway is disconnected.</div>
            </div>
            <label className="toggle">
              <input type="checkbox" checked={settings.enableDemoMode}
                onChange={e => setSettings(p => ({ ...p, enableDemoMode: e.target.checked }))} />
              <span className="track" /><span className="knob" />
            </label>
          </div>
        </div>
      </motion.div>
    </motion.div>
  );
}
