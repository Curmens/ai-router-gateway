import React, { useState } from 'react';
import { Link, Key, RefreshCw, CheckCircle, AlertTriangle, Settings as SettingsIcon } from 'lucide-react';

export default function Settings({ settings, setSettings, isOnline }) {
  const [testStatus, setTestStatus] = useState('idle');
  const [testResult, setTestResult] = useState('');

  const verifyConnection = async () => {
    setTestStatus('testing'); setTestResult('');
    try {
      const res = await fetch(`${settings.apiHost}/health`);
      if (res.ok) {
        const d = await res.json();
        setTestStatus('success');
        setTestResult(`Gateway connected — status: ${d.status}, env: ${d.environment}`);
      } else { throw new Error(`Code ${res.status}`); }
    } catch (err) {
      setTestStatus('error');
      setTestResult(`Connection failed. Verify gateway on ${settings.apiHost}`);
    }
  };

  return (
    <div className="dc-fade-in" style={{ display: 'flex', flexDirection: 'column', gap: '16px', maxWidth: 740 }}>

      {/* ── Connection Section ── */}
      <div className="card">
        <div className="card-header" style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <Link size={12} style={{ color: 'var(--dc-brand-500)' }} />
          Gateway Connection
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>

          <div>
            <label className="dc-label">API Endpoint</label>
            <input
              className="dc-input"
              type="text"
              value={settings.apiHost}
              onChange={e => setSettings(p => ({ ...p, apiHost: e.target.value }))}
              style={{ fontFamily: 'var(--dc-font-code)' }}
            />
          </div>

          <div>
            <label className="dc-label">Authorization Token</label>
            <div style={{ position: 'relative' }}>
              <Key size={14} style={{ position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)', color: 'var(--dc-text-muted)' }} />
              <input
                className="dc-input"
                type="password"
                value={settings.apiKey}
                onChange={e => setSettings(p => ({ ...p, apiKey: e.target.value }))}
                style={{ paddingLeft: 36, fontFamily: 'var(--dc-font-code)' }}
              />
            </div>
          </div>

          <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
            <button className="dc-btn dc-btn-primary" onClick={verifyConnection} disabled={testStatus === 'testing'}>
              {testStatus === 'testing' && <RefreshCw size={13} className="dc-spin" />}
              Verify Connection
            </button>
            <span style={{ fontSize: 13, fontWeight: 700, color: isOnline ? 'var(--dc-green-360)' : 'var(--dc-red-400)' }}>
              {isOnline ? '● Online' : '● Offline'}
            </span>
          </div>

          {testStatus === 'success' && (
            <div className="card-embed dc-fade-in" style={{ borderLeftColor: 'var(--dc-green-360)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, color: 'var(--dc-green-360)', fontSize: 14 }}>
                <CheckCircle size={14} />
                {testResult}
              </div>
            </div>
          )}
          {testStatus === 'error' && (
            <div className="card-embed dc-fade-in" style={{ borderLeftColor: 'var(--dc-red-400)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, color: 'var(--dc-red-400)', fontSize: 14 }}>
                <AlertTriangle size={14} />
                {testResult}
              </div>
            </div>
          )}
        </div>
      </div>

      {/* ── Dashboard Config ── */}
      <div className="card">
        <div className="card-header" style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <SettingsIcon size={12} style={{ color: 'var(--dc-fuchsia-400)' }} />
          Dashboard Configuration
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 0 }}>

          {/* Polling Speed */}
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '14px 0' }}>
            <div>
              <div style={{ fontSize: 15, fontWeight: 600, color: 'var(--dc-text-header)' }}>
                Console Polling Speed
              </div>
              <div style={{ fontSize: 13, color: 'var(--dc-text-muted)', marginTop: 2 }}>
                How frequently telemetry is refreshed.
              </div>
            </div>
            <select
              className="dc-select"
              value={settings.pollInterval}
              onChange={e => setSettings(p => ({ ...p, pollInterval: parseInt(e.target.value, 10) }))}
            >
              <option value="1000">Fast (1.0s)</option>
              <option value="2000">Normal (2.0s)</option>
              <option value="5000">Slow (5.0s)</option>
            </select>
          </div>

          <div className="dc-divider" />

          {/* Demo Mode */}
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '14px 0' }}>
            <div>
              <div style={{ fontSize: 15, fontWeight: 600, color: 'var(--dc-text-header)' }}>
                Offline Simulation
              </div>
              <div style={{ fontSize: 13, color: 'var(--dc-text-muted)', marginTop: 2 }}>
                Enable mock request flows when gateway is disconnected.
              </div>
            </div>
            <label className="dc-toggle">
              <input
                type="checkbox"
                checked={settings.enableDemoMode}
                onChange={e => setSettings(p => ({ ...p, enableDemoMode: e.target.checked }))}
              />
              <span className="track" />
              <span className="thumb" />
            </label>
          </div>
        </div>
      </div>
    </div>
  );
}
