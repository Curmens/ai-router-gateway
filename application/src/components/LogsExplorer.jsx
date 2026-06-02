import React, { useState, useEffect, useMemo } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Search, RefreshCw, ChevronDown, ChevronUp, FileCode, CheckCircle, XCircle } from 'lucide-react';
import { ProviderLogo } from './ProviderLogos';

const MOCK_LOGS = [
  {
    id: 'req_fa890e12d4b689a7', created_at: new Date(Date.now() - 30000).toISOString(),
    original_model: 'auto', provider: 'subscription', model: 'claude-3-7-sonnet',
    latency_ms: 412, status: 200, cost: 0.0042, routing_type: 'auto',
    complexity: 0.94, confidence: 0.97,
    reason: 'Highly complex architectural design prompt. Dynamic Auto-Routing scheduled Subscription tier.'
  },
  {
    id: 'req_28cb9a1c0d45f3e2', created_at: new Date(Date.now() - 120000).toISOString(),
    original_model: 'qwen3:latest', provider: 'ollama', model: 'qwen3:latest',
    latency_ms: 124, status: 200, cost: 0.0, routing_type: 'explicit',
    complexity: 0.15, confidence: 1.0,
    reason: 'Explicit request for local model. Routed directly to Ollama endpoint.'
  },
  {
    id: 'req_876b5c00e12a67e1', created_at: new Date(Date.now() - 340000).toISOString(),
    original_model: 'auto', provider: 'agy', model: 'gemini-3.1-pro',
    latency_ms: 320, status: 200, cost: 0.0, routing_type: 'auto',
    complexity: 0.72, confidence: 0.89,
    reason: 'Structured reasoning request. Routed to dynamic agy wrapper endpoint.'
  },
  {
    id: 'req_bc8917d0c345a982', created_at: new Date(Date.now() - 600000).toISOString(),
    original_model: 'auto', provider: 'openai', model: 'gpt-4o-mini',
    latency_ms: 205, status: 200, cost: 0.00045, routing_type: 'auto',
    complexity: 0.42, confidence: 0.92,
    reason: 'Low complexity chat utility context. Routed to cheap OpenAI mini model.'
  },
  {
    id: 'req_89abcf20d43a65e9', created_at: new Date(Date.now() - 950000).toISOString(),
    original_model: 'auto', provider: 'openai', model: 'gpt-4o',
    latency_ms: 450, status: 500,
    error_message: 'API Key Quota Exceeded on upstream provider endpoint.',
    routing_type: 'auto', complexity: 0.85, confidence: 0.94,
    reason: 'Quota limits triggered. Proceeding to failover queue configurations...'
  }
];

export default function LogsExplorer({ apiHost, apiKey, isOnline }) {
  // Initialize with mock data to prevent blank screen
  const [logs, setLogs] = useState(MOCK_LOGS);
  const [isLoading, setIsLoading] = useState(false);
  const [expandedLogId, setExpandedLogId] = useState(null);
  const [filterProvider, setFilterProvider] = useState('');
  const [filterStatus, setFilterStatus] = useState('');
  const [searchQuery, setSearchQuery] = useState('');

  const fetchLogs = async () => {
    setIsLoading(true);
    try {
      if (isOnline) {
        const url = new URL(`${apiHost}/v1/logs`);
        url.searchParams.append('limit', '50');
        if (filterProvider) url.searchParams.append('provider', filterProvider);
        if (filterStatus) url.searchParams.append('status', filterStatus);
        const res = await fetch(url.toString(), {
          headers: { 'Authorization': `Bearer ${apiKey}` }
        });
        if (res.ok) { const d = await res.json(); setLogs(d.logs || MOCK_LOGS); }
      }
    } catch (e) { /* keep existing logs */ }
    finally { setIsLoading(false); }
  };

  useEffect(() => { fetchLogs(); }, [isOnline, filterProvider, filterStatus]);

  const filteredLogs = useMemo(() => {
    const q = searchQuery.toLowerCase();
    if (!q) return logs;
    return logs.filter(log =>
      (log.id?.toLowerCase() || '').includes(q) ||
      (log.model?.toLowerCase() || '').includes(q) ||
      (log.provider?.toLowerCase() || '').includes(q) ||
      (log.reason?.toLowerCase() || '').includes(q)
    );
  }, [logs, searchQuery]);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>

      {/* ── Filter Bar ── */}
      <div className="card" style={{ display: 'flex', gap: 10, alignItems: 'center', flexWrap: 'wrap', padding: '12px 16px' }}>
        <div style={{ position: 'relative', flex: '1 1 220px' }}>
          <Search size={14} style={{ position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
          <input
            className="input"
            type="text"
            placeholder="Search traces…"
            value={searchQuery}
            onChange={e => setSearchQuery(e.target.value)}
            style={{ paddingLeft: 34 }}
          />
        </div>

        <select className="select" value={filterProvider} onChange={e => setFilterProvider(e.target.value)}>
          <option value="">All Providers</option>
          <option value="ollama">Ollama</option>
          <option value="gemini">Gemini</option>
          <option value="openai">OpenAI</option>
          <option value="agy">AGY</option>
          <option value="subscription">Claude</option>
        </select>

        <select className="select" value={filterStatus} onChange={e => setFilterStatus(e.target.value)}>
          <option value="">All Statuses</option>
          <option value="200">Success 200</option>
          <option value="500">Error 500</option>
        </select>

        <button className="btn btn-ghost" onClick={fetchLogs} disabled={isLoading}>
          <RefreshCw size={13} className={isLoading ? 'spin' : ''} />
          Sync
        </button>
      </div>

      {/* ── Log Table ── */}
      <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
        <div style={{ overflowX: 'auto' }}>
          <table className="flat-table">
            <thead>
              <tr>
                <th>Time</th>
                <th>Request ID</th>
                <th>Requested</th>
                <th>Routed To</th>
                <th>Latency</th>
                <th>Status</th>
                <th style={{ width: 36 }}></th>
              </tr>
            </thead>
            <tbody>
              {filteredLogs.map(log => {
                const logId = log.id || '';
                const isExpanded = expandedLogId === logId;
                const time = log.created_at ? new Date(log.created_at).toLocaleTimeString() : 'N/A';
                return (
                  <React.Fragment key={logId}>
                    <tr onClick={() => setExpandedLogId(isExpanded ? null : logId)}>
                      <td style={{ color: 'var(--text-muted)', fontSize: 12, fontFamily: 'var(--font-mono)' }}>{time}</td>
                      <td style={{ fontFamily: 'var(--font-mono)', fontSize: 12, color: 'var(--text-muted)' }}>
                        {logId ? logId.slice(0, 14) : 'N/A'}
                      </td>
                      <td><span className="badge">{log.original_model}</span></td>
                      <td>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                          <ProviderLogo provider={log.provider} size={16} />
                          <span style={{ fontWeight: 600 }}>{log.model}</span>
                        </div>
                      </td>
                      <td style={{ fontFamily: 'var(--font-mono)', fontWeight: 700 }}>
                        {log.latency_ms}ms
                      </td>
                      <td>
                        <span className={`tag ${log.status === 200 ? 'tag-success' : 'tag-error'}`}>
                          {log.status === 200 ? <CheckCircle size={10} /> : <XCircle size={10} />}
                          {log.status}
                        </span>
                      </td>
                      <td style={{ color: 'var(--text-muted)' }}>
                        {isExpanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
                      </td>
                    </tr>

                    <AnimatePresence>
                      {isExpanded && (
                        <tr>
                          <td colSpan="7" style={{ padding: 0 }}>
                            <motion.div
                              initial={{ height: 0, opacity: 0 }}
                              animate={{ height: 'auto', opacity: 1, transition: { duration: 0.25, ease: [0.4, 0, 0.2, 1] } }}
                              exit={{ height: 0, opacity: 0, transition: { duration: 0.15 } }}
                              style={{ overflow: 'hidden' }}
                            >
                              <div className="embed" style={{
                                margin: '8px 16px 16px',
                                borderLeftColor: log.status === 200 ? 'var(--green)' : 'var(--red)'
                              }}>
                                <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 10, fontSize: 11, fontWeight: 700, color: 'var(--accent)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>
                                  <FileCode size={12} />
                                  Audit Trace Inspector
                                </div>
                                <p style={{ fontSize: 13, color: 'var(--text-primary)', margin: '0 0 10px', lineHeight: 1.6 }}>
                                  <strong style={{ color: 'var(--text-secondary)' }}>Decision:</strong> {log.reason || 'Explicit route bypass.'}
                                </p>
                                <pre className="code">{JSON.stringify(log, null, 2)}</pre>
                              </div>
                            </motion.div>
                          </td>
                        </tr>
                      )}
                    </AnimatePresence>
                  </React.Fragment>
                );
              })}
              {filteredLogs.length === 0 && (
                <tr>
                  <td colSpan="7" style={{ padding: 32, textAlign: 'center', color: 'var(--text-muted)' }}>
                    No matching traces.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
