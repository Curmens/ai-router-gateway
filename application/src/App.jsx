import React, { useState, useEffect } from 'react';
import {
  LayoutDashboard, Terminal, Cpu, Settings as SettingsIcon,
  Activity, RefreshCw, ShieldCheck, Hash, Search, Bell,
  HelpCircle, Pin
} from 'lucide-react';
import Dashboard from './components/Dashboard';
import LogsExplorer from './components/LogsExplorer';
import RouterCanvas from './components/RouterCanvas';
import Settings from './components/Settings';

const TABS = [
  { id: 'dashboard', label: 'overview',      icon: LayoutDashboard, desc: 'Real-time gateway telemetry and provider health' },
  { id: 'canvas',    label: 'router-path',   icon: Cpu,             desc: 'Live routing flow visualization and decision inspector' },
  { id: 'explorer',  label: 'logs-explorer',  icon: Terminal,        desc: 'Query and inspect request audit traces' },
  { id: 'settings',  label: 'settings',       icon: SettingsIcon,    desc: 'Gateway connection and dashboard configuration' },
];

export default function App() {
  const [activeTab, setActiveTab] = useState('dashboard');
  const [isOnline, setIsOnline] = useState(false);
  const [isPolling, setIsPolling] = useState(false);
  const [models, setModels] = useState([]);
  const [providers, setProviders] = useState([]);
  const [gatewayStats, setGatewayStats] = useState({
    avgLatency: 0, requestsCount: 0, throughput: 0,
    errorRate: 0, cost: 0, tokens: 0
  });

  const [settings, setSettings] = useState(() => {
    const saved = localStorage.getItem('router_desktop_settings');
    return saved ? JSON.parse(saved) : {
      apiHost: 'http://localhost:8080',
      apiKey: 'sk-router-admin-12345',
      pollInterval: 2000,
      enableDemoMode: true
    };
  });

  useEffect(() => {
    localStorage.setItem('router_desktop_settings', JSON.stringify(settings));
  }, [settings]);

  /* ── Polling ── */
  useEffect(() => {
    let timer;

    async function pollGateway() {
      if (isPolling) return;
      setIsPolling(true);
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 1200);

      try {
        const healthRes = await fetch(`${settings.apiHost}/health`, { signal: controller.signal });
        if (!healthRes.ok) throw new Error('Unhealthy');
        setIsOnline(true);
        clearTimeout(timeoutId);

        const modelsRes = await fetch(`${settings.apiHost}/v1/models`, {
          headers: { 'Authorization': `Bearer ${settings.apiKey}` }
        });
        if (modelsRes.ok) { const d = await modelsRes.json(); setModels(d.data || []); }

        const providersRes = await fetch(`${settings.apiHost}/v1/providers`, {
          headers: { 'Authorization': `Bearer ${settings.apiKey}` }
        });
        if (providersRes.ok) { const d = await providersRes.json(); setProviders(d || []); }

        const usageRes = await fetch(`${settings.apiHost}/v1/usage/logs?limit=1`, {
          headers: { 'Authorization': `Bearer ${settings.apiKey}` }
        });
        if (usageRes.ok) {
          const d = await usageRes.json();
          if (d.summary) {
            setGatewayStats(prev => ({
              ...prev,
              requestsCount: d.summary.request_count || 0,
              cost: d.summary.total_cost || 0.0,
              tokens: d.summary.total_tokens || 0
            }));
          }
        }

        const logsRes = await fetch(`${settings.apiHost}/v1/logs?limit=50`, {
          headers: { 'Authorization': `Bearer ${settings.apiKey}` }
        });
        if (logsRes.ok) {
          const d = await logsRes.json();
          if (d.logs?.length > 0) {
            const logs = d.logs;
            const sumLatency = logs.reduce((a, l) => a + (l.latency_ms || 0), 0);
            const errCount = logs.filter(l => l.status !== 200).length;
            setGatewayStats(prev => ({
              ...prev,
              avgLatency: Math.round(sumLatency / logs.length),
              errorRate: Math.round((errCount / logs.length) * 100),
              throughput: Math.round((logs.length / 60) * 10) / 10
            }));
          }
        }
      } catch {
        setIsOnline(false);
        if (settings.enableDemoMode) loadMockTelemetry();
      } finally {
        setIsPolling(false);
      }
    }

    function loadMockTelemetry() {
      setModels([
        { id: 'gemini-3.5-flash', owned_by: 'agy' },
        { id: 'gemini-3.1-pro', owned_by: 'agy' },
        { id: 'claude-3-7-sonnet', owned_by: 'subscription' },
        { id: 'claude-3-5-sonnet', owned_by: 'subscription' },
        { id: 'qwen2.5:latest', owned_by: 'ollama' },
        { id: 'gpt-4o', owned_by: 'openai' },
        { id: 'gpt-4o-mini', owned_by: 'openai' }
      ]);
      setProviders([
        { name: 'ollama', is_healthy: true, avg_latency: 180 },
        { name: 'gemini', is_healthy: true, avg_latency: 240 },
        { name: 'subscription', is_healthy: true, avg_latency: 410 },
        { name: 'agy', is_healthy: true, avg_latency: 350 },
        { name: 'openai', is_healthy: false, avg_latency: 0 }
      ]);
      setGatewayStats(prev => {
        const d = Math.random() > 0.5 ? 1 : -1;
        return {
          avgLatency: prev.avgLatency === 0 ? 285 : Math.max(120, Math.min(480, prev.avgLatency + d * Math.floor(Math.random() * 8))),
          requestsCount: prev.requestsCount === 0 ? 4720 : prev.requestsCount + (Math.random() > 0.75 ? 1 : 0),
          throughput: Math.max(1.2, Math.min(8.5, (prev.throughput || 3.4) + d * 0.2)),
          errorRate: 1,
          cost: prev.cost === 0 ? 12.84 : prev.cost + (Math.random() > 0.9 ? 0.002 : 0),
          tokens: prev.tokens === 0 ? 1492000 : prev.tokens + (Math.random() > 0.75 ? 420 : 0)
        };
      });
    }

    pollGateway();
    timer = setInterval(pollGateway, settings.pollInterval);
    return () => clearInterval(timer);
  }, [settings.apiHost, settings.apiKey, settings.pollInterval, settings.enableDemoMode]);

  const currentTab = TABS.find(t => t.id === activeTab);

  return (
    <div className="app-shell">

      {/* ── Custom Titlebar ── */}
      <div className="titlebar">
        <span>ROUTER DESKTOP</span>
      </div>

      {/* ── 2-Column Body ── */}
      <div className="app-body">

        {/* ── Sidebar ── */}
        <div className="sidebar">
          <div className="sidebar-header">
            <Activity size={18} style={{ color: 'var(--dc-brand-500)' }} />
            <span className="server-name">AI Router</span>
          </div>

          <nav className="channel-list">
            <div className="channel-category">Monitoring</div>
            {TABS.slice(0, 3).map(tab => (
              <button
                key={tab.id}
                className={`channel-item ${activeTab === tab.id ? 'active' : ''}`}
                onClick={() => setActiveTab(tab.id)}
              >
                <span className="hash">#</span>
                {tab.label}
              </button>
            ))}

            <div className="channel-category">System</div>
            {TABS.slice(3).map(tab => (
              <button
                key={tab.id}
                className={`channel-item ${activeTab === tab.id ? 'active' : ''}`}
                onClick={() => setActiveTab(tab.id)}
              >
                <span className="hash">#</span>
                {tab.label}
              </button>
            ))}
          </nav>

          {/* ── User Panel ── */}
          <div className="user-panel">
            <div className="user-avatar">
              <div className="ring" style={{ background: 'var(--dc-brand-500)' }}>R</div>
              <span className={`status-badge ${isOnline ? 'online' : 'offline'}`} />
            </div>
            <div className="user-info">
              <div className="username">{isOnline ? 'Gateway Online' : 'Offline Mode'}</div>
              <div className="status-text">{settings.apiHost.replace('http://', '')}</div>
            </div>
          </div>
        </div>

        {/* ── Main Region ── */}
        <div className="main-region">

          {/* Top Bar */}
          <div className="topbar">
            <div style={{ display: 'flex', alignItems: 'center' }}>
              <div className="topbar-title">
                <span className="hash-icon">#</span>
                {currentTab?.label}
              </div>
              <div className="topbar-divider" />
              <span className="topbar-desc">{currentTab?.desc}</span>
            </div>

            <div className="topbar-actions">
              {isPolling && <RefreshCw size={14} className="dc-spin" style={{ color: 'var(--dc-brand-500)' }} />}
              <button className="topbar-btn" title="Search">
                <Search size={18} />
              </button>
              <button className="topbar-btn" title="Notifications">
                <Bell size={18} />
              </button>
              <button className="topbar-btn" title="Help">
                <HelpCircle size={18} />
              </button>
            </div>
          </div>

          {/* Content Scroll */}
          <div className="content-scroll">
            {activeTab === 'dashboard' && (
              <Dashboard stats={gatewayStats} providers={providers} models={models} isOnline={isOnline} />
            )}
            {activeTab === 'canvas' && (
              <RouterCanvas providers={providers} models={models} apiHost={settings.apiHost} apiKey={settings.apiKey} isOnline={isOnline} />
            )}
            {activeTab === 'explorer' && (
              <LogsExplorer apiHost={settings.apiHost} apiKey={settings.apiKey} isOnline={isOnline} />
            )}
            {activeTab === 'settings' && (
              <Settings settings={settings} setSettings={setSettings} isOnline={isOnline} />
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
