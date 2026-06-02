import React, { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  LayoutDashboard, Terminal, Cpu, Settings as SettingsIcon,
  RefreshCw, Search, Bell, HelpCircle, Activity, MessageSquare
} from 'lucide-react';
import Preloader from './components/Preloader';
import Dashboard from './components/Dashboard';
import LogsExplorer from './components/LogsExplorer';
import RouterView from './components/RouterView';
import Settings from './components/Settings';
import Playground from './components/Playground';

const TABS = [
  { id: 'dashboard', label: 'Overview',      icon: LayoutDashboard, desc: 'Real-time gateway telemetry and provider health' },
  { id: 'chat',      label: 'Playground',    icon: MessageSquare,   desc: 'Interactive chat client routed through gateway' },
  { id: 'canvas',    label: 'Router Flow',   icon: Cpu,             desc: 'Live 3D routing flow visualization' },
  { id: 'explorer',  label: 'Logs',          icon: Terminal,        desc: 'Query and inspect request audit traces' },
  { id: 'settings',  label: 'Settings',      icon: SettingsIcon,    desc: 'Gateway connection and provider configuration' },
];

const pageVariants = {
  initial: { opacity: 0, y: 12 },
  animate: { opacity: 1, y: 0, transition: { duration: 0.3, ease: [0.4, 0, 0.2, 1] } },
  exit:    { opacity: 0, y: -8, transition: { duration: 0.15 } }
};

export default function App() {
  const [loading, setLoading] = useState(true);
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
      } catch (e) {
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

  if (loading) {
    return <Preloader onComplete={() => setLoading(false)} />;
  }

  return (
    <div className="app-shell">
      {/* Titlebar */}
      <div className="titlebar">
        <span>AI ROUTER</span>
      </div>

      <div className="app-body">
        {/* ── Sidebar ── */}
        <div className="sidebar">
          <div className="sidebar-header">
            <Activity size={20} style={{ color: 'var(--accent)' }} />
            <span className="sidebar-brand">
              AI <span className="sidebar-accent">Router</span>
            </span>
          </div>

          <nav className="channel-list">
            <div className="channel-category">Monitoring</div>
            {TABS.slice(0, 4).map(tab => {
              const Icon = tab.icon;
              return (
                <button
                  key={tab.id}
                  className={`channel-item ${activeTab === tab.id ? 'active' : ''}`}
                  onClick={() => setActiveTab(tab.id)}
                >
                  <Icon size={16} className="channel-icon" />
                  {tab.label}
                </button>
              );
            })}

            <div className="channel-category">System</div>
            {TABS.slice(4).map(tab => {
              const Icon = tab.icon;
              return (
                <button
                  key={tab.id}
                  className={`channel-item ${activeTab === tab.id ? 'active' : ''}`}
                  onClick={() => setActiveTab(tab.id)}
                >
                  <Icon size={16} className="channel-icon" />
                  {tab.label}
                </button>
              );
            })}
          </nav>

          <div className="user-panel">
            <div className="user-avatar-ring">
              R
              <span className={`status-dot ${isOnline ? 'online' : 'offline'}`} />
            </div>
            <div>
              <div className="user-name">{isOnline ? 'Gateway Online' : 'Offline Mode'}</div>
              <div className="user-sub">{settings.apiHost.replace('http://', '')}</div>
            </div>
          </div>
        </div>

        {/* ── Main Region ── */}
        <div className="main-region">
          <div className="topbar">
            <div style={{ display: 'flex', alignItems: 'center' }}>
              <span className="topbar-title">{currentTab?.label}</span>
              <span className="topbar-desc">{currentTab?.desc}</span>
            </div>
            <div className="topbar-actions">
              {isPolling && <RefreshCw size={14} className="spin" style={{ color: 'var(--accent)' }} />}
              <button className="topbar-btn"><Search size={16} /></button>
              <button className="topbar-btn"><Bell size={16} /></button>
              <button className="topbar-btn"><HelpCircle size={16} /></button>
            </div>
          </div>

          <div className="content-scroll">
            <AnimatePresence mode="wait">
              <motion.div
                key={activeTab}
                variants={pageVariants}
                initial="initial"
                animate="animate"
                exit="exit"
              >
                {activeTab === 'dashboard' && (
                  <Dashboard stats={gatewayStats} providers={providers} models={models} isOnline={isOnline} />
                )}
                {activeTab === 'chat' && (
                  <Playground apiHost={settings.apiHost} apiKey={settings.apiKey} isOnline={isOnline} models={models} />
                )}
                {activeTab === 'canvas' && (
                  <RouterView providers={providers} models={models} apiHost={settings.apiHost} apiKey={settings.apiKey} isOnline={isOnline} />
                )}
                {activeTab === 'explorer' && (
                  <LogsExplorer apiHost={settings.apiHost} apiKey={settings.apiKey} isOnline={isOnline} />
                )}
                {activeTab === 'settings' && (
                  <Settings settings={settings} setSettings={setSettings} isOnline={isOnline} />
                )}
              </motion.div>
            </AnimatePresence>
          </div>
        </div>
      </div>
    </div>
  );
}
