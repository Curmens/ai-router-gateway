import React from 'react';
import { Activity, Cpu, DollarSign, Layers, Zap, AlertTriangle } from 'lucide-react';

const PROV_COLORS = {
  openai: 'var(--prov-openai)',
  gemini: 'var(--prov-gemini)',
  ollama: 'var(--prov-ollama)',
  agy: 'var(--prov-agy)',
  subscription: 'var(--prov-subscription)',
};
const provColor = (name) => PROV_COLORS[name?.toLowerCase()] || 'var(--dc-brand-500)';

export default function Dashboard({ stats, providers, models, isOnline }) {
  return (
    <div className="dc-fade-in" style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>

      {/* ── Stat Cards ── */}
      <div className="stat-grid">

        <div className="stat-card">
          <div className="stat-icon-wrap" style={{ background: 'rgba(88,101,242,0.12)' }}>
            <Activity size={20} style={{ color: 'var(--dc-brand-500)' }} />
          </div>
          <div>
            <div className="stat-label">Avg Latency</div>
            <div className="stat-value">
              {stats.avgLatency}<span className="stat-unit">ms</span>
            </div>
          </div>
        </div>

        <div className="stat-card">
          <div className="stat-icon-wrap" style={{ background: 'rgba(235,69,158,0.12)' }}>
            <Zap size={20} style={{ color: 'var(--dc-fuchsia-400)' }} />
          </div>
          <div>
            <div className="stat-label">Throughput</div>
            <div className="stat-value">
              {stats.throughput}<span className="stat-unit">req/s</span>
            </div>
          </div>
        </div>

        <div className="stat-card">
          <div className="stat-icon-wrap" style={{ background: 'rgba(35,165,90,0.12)' }}>
            <Layers size={20} style={{ color: 'var(--dc-green-360)' }} />
          </div>
          <div>
            <div className="stat-label">Total Tokens</div>
            <div className="stat-value">
              {(stats.tokens / 1000).toFixed(1)}<span className="stat-unit">k</span>
            </div>
          </div>
        </div>

        <div className="stat-card">
          <div className="stat-icon-wrap" style={{ background: 'rgba(240,178,50,0.12)' }}>
            <DollarSign size={20} style={{ color: 'var(--dc-yellow-300)' }} />
          </div>
          <div>
            <div className="stat-label">Total Cost</div>
            <div className="stat-value" style={{ color: 'var(--dc-yellow-300)' }}>
              ${stats.cost.toFixed(4)}
            </div>
          </div>
        </div>
      </div>

      {/* ── Two-Panel: Providers + Models ── */}
      <div style={{ display: 'grid', gridTemplateColumns: '1.2fr 1fr', gap: '16px' }}>

        {/* Provider Health */}
        <div className="card">
          <div className="card-header">Active Service Providers</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
            {providers.map(p => (
              <div className="provider-row" key={p.name}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                  <div
                    className="provider-dot"
                    style={{
                      background: p.is_healthy ? 'var(--dc-green-360)' : 'var(--dc-red-400)',
                      boxShadow: p.is_healthy
                        ? '0 0 6px var(--dc-green-360)'
                        : '0 0 6px var(--dc-red-400)'
                    }}
                  />
                  <span className="provider-name">{p.name}</span>
                </div>
                <span style={{
                  fontFamily: 'var(--dc-font-code)',
                  fontSize: '13px',
                  fontWeight: 700,
                  color: p.is_healthy ? provColor(p.name) : 'var(--dc-text-muted)'
                }}>
                  {p.is_healthy ? `${p.avg_latency} ms` : 'Offline'}
                </span>
              </div>
            ))}
            {providers.length === 0 && (
              <div style={{ padding: '20px', textAlign: 'center', color: 'var(--dc-text-muted)', fontSize: '14px' }}>
                Connecting to provider endpoints…
              </div>
            )}
          </div>
        </div>

        {/* Model Registry */}
        <div className="card">
          <div className="card-header">Model Registry ({models.length})</div>
          <div style={{
            display: 'flex', flexDirection: 'column', gap: '2px',
            maxHeight: '320px', overflowY: 'auto'
          }}>
            {models.map(m => (
              <div className="model-row" key={m.id}>
                <span className="model-id">{m.id}</span>
                <span className="badge" style={{ color: provColor(m.owned_by) }}>
                  {m.owned_by}
                </span>
              </div>
            ))}
            {models.length === 0 && (
              <div style={{ padding: '20px', textAlign: 'center', color: 'var(--dc-text-muted)', fontSize: '14px' }}>
                No registered models available.
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
