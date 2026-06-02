import React from 'react';
import { motion } from 'framer-motion';
import { Activity, Zap, Layers, DollarSign } from 'lucide-react';
import { ProviderLogo } from './ProviderLogos';

const PROV_COLORS = {
  openai: 'var(--prov-openai)', gemini: 'var(--prov-gemini)',
  ollama: 'var(--prov-ollama)', agy: 'var(--prov-agy)',
  subscription: 'var(--prov-anthropic)',
};
const pc = (n) => PROV_COLORS[n?.toLowerCase()] || 'var(--accent)';

const stagger = {
  animate: { transition: { staggerChildren: 0.06 } }
};
const fadeUp = {
  initial: { opacity: 0, y: 16 },
  animate: { opacity: 1, y: 0, transition: { duration: 0.4, ease: [0.4, 0, 0.2, 1] } }
};

export default function Dashboard({ stats, providers, models }) {
  return (
    <motion.div variants={stagger} initial="initial" animate="animate"
      style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>

      {/* ── Stat Cards ── */}
      <div className="stat-grid">
        {[
          { label: 'Avg Latency', value: stats.avgLatency, unit: 'ms', color: 'var(--accent)', icon: Activity },
          { label: 'Throughput', value: stats.throughput, unit: 'req/s', color: 'var(--purple)', icon: Zap },
          { label: 'Total Tokens', value: `${(stats.tokens / 1000).toFixed(1)}`, unit: 'k', color: 'var(--green)', icon: Layers },
          { label: 'Total Cost', value: `$${stats.cost.toFixed(4)}`, unit: '', color: 'var(--yellow)', icon: DollarSign },
        ].map((s, i) => {
          const Icon = s.icon;
          return (
            <motion.div key={i} variants={fadeUp} className="stat-card">
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                <div className="stat-label">{s.label}</div>
                <Icon size={16} style={{ color: s.color, opacity: 0.7 }} />
              </div>
              <div className="stat-value">
                {s.value}<span className="stat-unit">{s.unit}</span>
              </div>
              <div className="stat-accent" style={{ background: `linear-gradient(90deg, ${s.color}, transparent)` }} />
            </motion.div>
          );
        })}
      </div>

      {/* ── Two Panels ── */}
      <div style={{ display: 'grid', gridTemplateColumns: '1.2fr 1fr', gap: 16 }}>

        {/* Provider Health */}
        <motion.div variants={fadeUp} className="card">
          <div className="card-header">Active Providers</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            {providers.map(p => {
              const maxLat = 500;
              const pct = p.is_healthy ? Math.min(p.avg_latency / maxLat * 100, 100) : 0;
              return (
                <div className="provider-row" key={p.name}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                    <div className="provider-logo-wrap">
                      <ProviderLogo provider={p.name} size={22} />
                    </div>
                    <div>
                      <span className="provider-name">{p.name}</span>
                      <div style={{ fontSize: 11, color: p.is_healthy ? 'var(--green)' : 'var(--text-muted)', fontWeight: 600, marginTop: 1 }}>
                        {p.is_healthy ? 'Online' : 'Offline'}
                      </div>
                    </div>
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                    <div className="latency-bar-track">
                      <div className="latency-bar-fill" style={{
                        width: `${pct}%`,
                        background: p.is_healthy ? pc(p.name) : 'var(--text-dim)'
                      }} />
                    </div>
                    <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12, fontWeight: 700, color: p.is_healthy ? '#fff' : 'var(--text-muted)', minWidth: 50, textAlign: 'right' }}>
                      {p.is_healthy ? `${p.avg_latency}ms` : '—'}
                    </span>
                  </div>
                </div>
              );
            })}
            {providers.length === 0 && (
              <div style={{ padding: 24, textAlign: 'center', color: 'var(--text-muted)', fontSize: 13 }}>
                Connecting to providers…
              </div>
            )}
          </div>
        </motion.div>

        {/* Model Registry */}
        <motion.div variants={fadeUp} className="card">
          <div className="card-header">Model Registry ({models.length})</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 2, maxHeight: 340, overflowY: 'auto' }}>
            {models.map(m => (
              <div className="model-row" key={m.id}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <ProviderLogo provider={m.owned_by} size={16} />
                  <span className="model-id">{m.id}</span>
                </div>
                <span className="badge" style={{ color: pc(m.owned_by) }}>{m.owned_by}</span>
              </div>
            ))}
            {models.length === 0 && (
              <div style={{ padding: 24, textAlign: 'center', color: 'var(--text-muted)', fontSize: 13 }}>
                No registered models.
              </div>
            )}
          </div>
        </motion.div>
      </div>
    </motion.div>
  );
}
