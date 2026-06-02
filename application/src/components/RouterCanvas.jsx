import React, { useRef, useEffect, useState } from 'react';
import { Sparkles } from 'lucide-react';

const PROV_COLORS = {
  openai: '#23a55a', gemini: '#5865f2', ollama: '#1abc9c',
  agy: '#5865f2', subscription: '#9b59b6',
};
const provColor = (n) => PROV_COLORS[n?.toLowerCase()] || '#5865f2';

export default function RouterCanvas({ providers, models, apiHost, apiKey, isOnline }) {
  const canvasRef = useRef(null);
  const animRef = useRef(null);

  const [latest, setLatest] = useState({
    prompt: 'Querying DB indexes optimization',
    originalModel: 'auto',
    chosenProvider: 'subscription',
    chosenModel: 'claude-3-7-sonnet',
    complexity: 0.91,
    confidence: 0.96,
    reason: 'High coding complexity requested. Routed to premium subscription tier.',
    timestamp: new Date().toLocaleTimeString(),
    status: 200
  });

  const nodesRef = useRef({ client: { x: 80, y: 180 }, router: { x: 260, y: 180 }, providers: {} });
  const particles = useRef([]);

  const mockPrompts = [
    { text: 'Optimizing high-throughput connection pools', provider: 'subscription', model: 'claude-3-7-sonnet', complexity: 0.94, confidence: 0.97, reason: 'High-reasoning prompt routed to Subscription model' },
    { text: 'Calculate average fibonacci sequence elements', provider: 'ollama', model: 'qwen3:latest', complexity: 0.12, confidence: 0.99, reason: 'Low complexity mathematical task routed to local Ollama instance' },
    { text: 'Drafting legal contract framework for tech startup', provider: 'agy', model: 'gemini-3.1-pro', complexity: 0.78, confidence: 0.91, reason: 'Medium-high drafting routed to premium agy provider' },
    { text: 'Summarize standard transcript file logs', provider: 'gemini', model: 'gemini-2.5-flash', complexity: 0.35, confidence: 0.88, reason: 'Context-heavy summarization routed to Gemini provider' },
    { text: 'Generate SQL relational table mapping schema', provider: 'openai', model: 'gpt-4o', complexity: 0.68, confidence: 0.93, reason: 'Structured relational mapping routed to OpenAI tier' }
  ];

  const triggerVisual = (name) => {
    const t = nodesRef.current.providers[name?.toLowerCase()];
    if (!t) return;
    for (let i = 0; i < 12; i++) {
      particles.current.push({
        x: nodesRef.current.client.x, y: nodesRef.current.client.y,
        targetX: nodesRef.current.router.x, targetY: nodesRef.current.router.y,
        speed: 2.2 + Math.random() * 2,
        color: '#dbdee1', size: 2 + Math.random() * 1.5,
        progress: 0, finalTarget: name.toLowerCase(), phase: 'to_router'
      });
    }
  };

  /* ── Data Polling / Mock ── */
  useEffect(() => {
    let iv;
    if (!isOnline) {
      iv = setInterval(() => {
        const rp = mockPrompts[Math.floor(Math.random() * mockPrompts.length)];
        setLatest({
          prompt: rp.text, originalModel: 'auto', chosenProvider: rp.provider,
          chosenModel: rp.model, complexity: rp.complexity, confidence: rp.confidence,
          reason: rp.reason, timestamp: new Date().toLocaleTimeString(), status: 200
        });
        triggerVisual(rp.provider);
      }, 3000);
    } else {
      let lastId = '';
      async function poll() {
        try {
          const res = await fetch(`${apiHost}/v1/logs?limit=1`, {
            headers: { 'Authorization': `Bearer ${apiKey}` }
          });
          if (res.ok) {
            const d = await res.json();
            if (d.logs?.[0] && d.logs[0].id !== lastId) {
              const log = d.logs[0];
              lastId = log.id;
              setLatest({
                prompt: log.original_model === 'orchestrated' ? 'Orchestrated dynamic agent task' : 'Completions chat endpoint call',
                originalModel: log.original_model || 'auto',
                chosenProvider: log.provider, chosenModel: log.model,
                complexity: log.complexity || 0.5, confidence: log.confidence || 1.0,
                reason: log.reason || 'Explicit routed path bypass',
                timestamp: new Date(log.created_at).toLocaleTimeString(),
                status: log.status
              });
              triggerVisual(log.provider);
            }
          }
        } catch {}
      }
      iv = setInterval(poll, 1500);
    }
    return () => clearInterval(iv);
  }, [isOnline, apiHost, apiKey]);

  /* ── Canvas Render Loop ── */
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');

    const resize = () => {
      canvas.width = canvas.parentElement.clientWidth;
      canvas.height = 360;
    };
    resize();

    const provList = ['ollama', 'gemini', 'agy', 'subscription', 'openai'];
    const cX = 80, rX = canvas.width * 0.38, pX = canvas.width * 0.78;

    nodesRef.current.client = { x: cX, y: canvas.height / 2 };
    nodesRef.current.router = { x: rX, y: canvas.height / 2 };
    provList.forEach((p, i) => {
      const sp = canvas.height / (provList.length + 1);
      nodesRef.current.providers[p] = { x: pX, y: sp * (i + 1), label: p.toUpperCase() };
    });

    const drawNode = (x, y, label, color) => {
      // Outer glow ring
      ctx.fillStyle = color + '18';
      ctx.beginPath(); ctx.arc(x, y, 14, 0, Math.PI * 2); ctx.fill();
      // Inner solid
      ctx.fillStyle = color;
      ctx.beginPath(); ctx.arc(x, y, 5, 0, Math.PI * 2); ctx.fill();
      // Label
      ctx.font = "700 10px 'Outfit', 'Inter', sans-serif";
      ctx.fillStyle = '#949ba4';
      ctx.textAlign = 'center';
      ctx.fillText(label, x, y - 22);
    };

    const render = () => {
      ctx.fillStyle = '#313338';
      ctx.fillRect(0, 0, canvas.width, canvas.height);

      // Subtle grid pattern
      ctx.strokeStyle = 'rgba(255,255,255,0.015)';
      ctx.lineWidth = 1;
      for (let gx = 0; gx < canvas.width; gx += 40) {
        ctx.beginPath(); ctx.moveTo(gx, 0); ctx.lineTo(gx, canvas.height); ctx.stroke();
      }
      for (let gy = 0; gy < canvas.height; gy += 40) {
        ctx.beginPath(); ctx.moveTo(0, gy); ctx.lineTo(canvas.width, gy); ctx.stroke();
      }

      // Connections
      ctx.lineWidth = 1;
      ctx.strokeStyle = 'rgba(255,255,255,0.04)';
      ctx.setLineDash([4, 4]);

      const n = nodesRef.current;
      ctx.beginPath(); ctx.moveTo(n.client.x, n.client.y); ctx.lineTo(n.router.x, n.router.y); ctx.stroke();

      Object.values(n.providers).forEach(prov => {
        ctx.beginPath(); ctx.moveTo(n.router.x, n.router.y);
        ctx.bezierCurveTo(
          (n.router.x + prov.x) / 2, n.router.y,
          (n.router.x + prov.x) / 2, prov.y,
          prov.x, prov.y
        );
        ctx.stroke();
      });
      ctx.setLineDash([]);

      // Particles
      for (let i = particles.current.length - 1; i >= 0; i--) {
        const p = particles.current[i];
        p.progress += p.speed * 0.006;

        if (p.phase === 'to_router') {
          p.x = n.client.x + (n.router.x - n.client.x) * p.progress;
          p.y = n.client.y + (n.router.y - n.client.y) * p.progress;
          if (p.progress >= 1) {
            p.phase = 'to_provider'; p.progress = 0;
            p.color = provColor(p.finalTarget);
            p.x = n.router.x; p.y = n.router.y;
            const t = n.providers[p.finalTarget];
            if (t) { p.targetX = t.x; p.targetY = t.y; }
            else { particles.current.splice(i, 1); continue; }
          }
        } else {
          const sx = n.router.x, sy = n.router.y, t = p.progress;
          const c1x = (sx + p.targetX) / 2, c1y = sy, c2x = c1x, c2y = p.targetY;
          p.x = (1-t)**3*sx + 3*(1-t)**2*t*c1x + 3*(1-t)*t*t*c2x + t**3*p.targetX;
          p.y = (1-t)**3*sy + 3*(1-t)**2*t*c1y + 3*(1-t)*t*t*c2y + t**3*p.targetY;
          if (p.progress >= 1) { particles.current.splice(i, 1); continue; }
        }

        // Particle glow
        ctx.shadowBlur = 6;
        ctx.shadowColor = p.color;
        ctx.fillStyle = p.color;
        ctx.beginPath(); ctx.arc(p.x, p.y, p.size, 0, Math.PI * 2); ctx.fill();
        ctx.shadowBlur = 0;
      }

      // Nodes
      drawNode(n.client.x, n.client.y, 'GATEWAY', '#dbdee1');
      drawNode(n.router.x, n.router.y, 'CLASSIFIER', '#eb459e');
      Object.entries(n.providers).forEach(([k, v]) => drawNode(v.x, v.y, v.label, provColor(k)));

      animRef.current = requestAnimationFrame(render);
    };

    render();
    return () => cancelAnimationFrame(animRef.current);
  }, []);

  return (
    <div className="dc-fade-in" style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>

      {/* Canvas */}
      <div className="card" style={{ padding: 0, overflow: 'hidden', position: 'relative' }}>
        <canvas ref={canvasRef} style={{ display: 'block', width: '100%', height: '360px' }} />
        <div style={{
          position: 'absolute', top: 12, left: 12,
          display: 'flex', gap: 6, alignItems: 'center',
          background: 'var(--dc-bg-tertiary)', padding: '4px 10px',
          borderRadius: 4, fontSize: 11, fontWeight: 700, color: 'var(--dc-text-muted)'
        }}>
          <Sparkles size={10} style={{ color: 'var(--dc-brand-500)' }} />
          LIVE FLOW
        </div>
      </div>

      {/* Metadata Embed */}
      <div className="card">
        <div className="card-header">Latest Transaction</div>

        <div style={{ display: 'grid', gridTemplateColumns: '1.4fr 1fr', gap: '16px' }}>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
            <div>
              <div className="card-header" style={{ marginBottom: 6 }}>Prompt</div>
              <div className="code-block">"{latest.prompt}"</div>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
              <div>
                <div className="card-header" style={{ marginBottom: 6 }}>Route</div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                  <span className="badge" style={{ color: provColor(latest.chosenProvider) }}>
                    {latest.chosenProvider}
                  </span>
                  <span style={{ fontWeight: 600, color: 'var(--dc-text-header)' }}>
                    {latest.chosenModel}
                  </span>
                </div>
              </div>
              <div>
                <div className="card-header" style={{ marginBottom: 6 }}>Classification</div>
                <div style={{ display: 'flex', gap: 12, fontSize: 13 }}>
                  <span>
                    <span style={{ color: 'var(--dc-text-muted)' }}>Cplx </span>
                    <span style={{ fontWeight: 700, color: 'var(--dc-yellow-300)' }}>
                      {(latest.complexity * 100).toFixed(0)}%
                    </span>
                  </span>
                  <span>
                    <span style={{ color: 'var(--dc-text-muted)' }}>Conf </span>
                    <span style={{ fontWeight: 700, color: 'var(--dc-green-360)' }}>
                      {(latest.confidence * 100).toFixed(0)}%
                    </span>
                  </span>
                </div>
              </div>
            </div>
          </div>

          <div className="card-embed" style={{ borderLeftColor: latest.status === 200 ? 'var(--dc-green-360)' : 'var(--dc-red-400)' }}>
            <div className="card-header" style={{ marginBottom: 6 }}>Router Reasoning</div>
            <p style={{ fontSize: 14, color: 'var(--dc-text-normal)', margin: 0, lineHeight: 1.5 }}>
              {latest.reason}
            </p>
            <div className="dc-divider" />
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span style={{ fontSize: 12, color: 'var(--dc-text-muted)' }}>{latest.timestamp}</span>
              <span className={`status-tag ${latest.status === 200 ? 'success' : 'error'}`}>
                HTTP {latest.status}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
