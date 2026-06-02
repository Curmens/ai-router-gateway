import React, { useRef, useMemo, useEffect, useState } from 'react';
import { Canvas, useFrame } from '@react-three/fiber';
import { OrbitControls, Text, Float } from '@react-three/drei';
import * as THREE from 'three';
import { Sparkles } from 'lucide-react';
import { ProviderLogo } from './ProviderLogos';

/* ── 3D Node Sphere ── */
function NodeSphere({ position, color, label, scale = 1 }) {
  const meshRef = useRef();
  const glowRef = useRef();

  useFrame((state) => {
    if (meshRef.current) {
      meshRef.current.rotation.y += 0.005;
    }
    if (glowRef.current) {
      glowRef.current.scale.setScalar(1 + Math.sin(state.clock.elapsedTime * 2) * 0.08);
    }
  });

  return (
    <group position={position}>
      {/* Outer glow */}
      <mesh ref={glowRef}>
        <sphereGeometry args={[0.35 * scale, 16, 16]} />
        <meshBasicMaterial color={color} transparent opacity={0.06} />
      </mesh>
      {/* Core sphere */}
      <mesh ref={meshRef}>
        <icosahedronGeometry args={[0.18 * scale, 1]} />
        <meshStandardMaterial
          color={color}
          emissive={color}
          emissiveIntensity={0.5}
          roughness={0.3}
          metalness={0.8}
        />
      </mesh>
      {/* Label */}
      <Text
        position={[0, -0.55 * scale, 0]}
        fontSize={0.12}
        color="#666666"
        anchorX="center"
        anchorY="top"
        letterSpacing={0.08}
      >
        {label}
      </Text>
    </group>
  );
}

/* ── Animated Connection Line ── */
function ConnectionLine({ start, end, color = '#ffffff', opacity = 0.06 }) {
  const points = useMemo(() => {
    const mid = new THREE.Vector3().lerpVectors(
      new THREE.Vector3(...start),
      new THREE.Vector3(...end),
      0.5
    );
    mid.y += 0.3;
    const curve = new THREE.QuadraticBezierCurve3(
      new THREE.Vector3(...start),
      mid,
      new THREE.Vector3(...end)
    );
    return curve.getPoints(32);
  }, [start, end]);

  return (
    <line>
      <bufferGeometry>
        <bufferAttribute
          attach="attributes-position"
          count={points.length}
          array={new Float32Array(points.flatMap(p => [p.x, p.y, p.z]))}
          itemSize={3}
        />
      </bufferGeometry>
      <lineBasicMaterial color={color} transparent opacity={opacity} />
    </line>
  );
}

/* ── Flowing Particle ── */
function FlowParticle({ start, end, color, delay = 0 }) {
  const ref = useRef();
  const curve = useMemo(() => {
    const mid = new THREE.Vector3().lerpVectors(
      new THREE.Vector3(...start),
      new THREE.Vector3(...end),
      0.5
    );
    mid.y += 0.3 + Math.random() * 0.2;
    return new THREE.QuadraticBezierCurve3(
      new THREE.Vector3(...start),
      mid,
      new THREE.Vector3(...end)
    );
  }, [start, end]);

  useFrame((state) => {
    if (!ref.current) return;
    const t = ((state.clock.elapsedTime * 0.3 + delay) % 1);
    const pos = curve.getPoint(t);
    ref.current.position.copy(pos);
    ref.current.material.opacity = Math.sin(t * Math.PI) * 0.9;
  });

  return (
    <mesh ref={ref}>
      <sphereGeometry args={[0.03, 8, 8]} />
      <meshBasicMaterial color={color} transparent opacity={0.5} />
    </mesh>
  );
}

/* ── Scene Content ── */
function RouterScene({ activeProvider }) {
  const provPositions = {
    ollama:       [-2.4, 1.2, 0],
    gemini:       [-1.2, 1.8, 0.5],
    agy:          [0,    2.0, -0.3],
    subscription: [1.2,  1.8, 0.5],
    openai:       [2.4,  1.2, 0],
  };

  const provColors = {
    ollama: '#ffffff',
    gemini: '#4285f4',
    agy: '#276ef1',
    subscription: '#d4a574',
    openai: '#00a67e',
  };

  const gatewayPos = [0, -1.5, 0];
  const routerPos = [0, 0, 0];

  return (
    <>
      {/* Lighting */}
      <ambientLight intensity={0.15} />
      <pointLight position={[0, 3, 5]} intensity={0.4} color="#276ef1" />
      <pointLight position={[-3, -2, 3]} intensity={0.2} color="#a855f7" />

      {/* Gateway Node */}
      <Float speed={1.5} rotationIntensity={0.1} floatIntensity={0.3}>
        <NodeSphere position={gatewayPos} color="#ffffff" label="GATEWAY" scale={1.2} />
      </Float>

      {/* Router / Classifier Node */}
      <Float speed={2} rotationIntensity={0.15} floatIntensity={0.4}>
        <NodeSphere position={routerPos} color="#a855f7" label="CLASSIFIER" scale={1.4} />
      </Float>

      {/* Connection: Gateway → Router */}
      <ConnectionLine start={gatewayPos} end={routerPos} color="#ffffff" opacity={0.08} />
      {[0, 0.33, 0.66].map((d, i) => (
        <FlowParticle key={`gw-${i}`} start={gatewayPos} end={routerPos} color="#ffffff" delay={d} />
      ))}

      {/* Provider Nodes + Connections */}
      {Object.entries(provPositions).map(([name, pos]) => {
        const isActive = activeProvider === name;
        return (
          <React.Fragment key={name}>
            <Float speed={1.5 + Math.random()} rotationIntensity={0.1} floatIntensity={0.2}>
              <NodeSphere
                position={pos}
                color={provColors[name]}
                label={name.toUpperCase()}
                scale={isActive ? 1.3 : 0.9}
              />
            </Float>
            <ConnectionLine start={routerPos} end={pos} color={provColors[name]} opacity={isActive ? 0.15 : 0.04} />
            {isActive && [0, 0.25, 0.5, 0.75].map((d, i) => (
              <FlowParticle key={`${name}-${i}`} start={routerPos} end={pos} color={provColors[name]} delay={d} />
            ))}
          </React.Fragment>
        );
      })}

      <OrbitControls
        enableZoom={false}
        enablePan={false}
        autoRotate
        autoRotateSpeed={0.5}
        maxPolarAngle={Math.PI / 1.8}
        minPolarAngle={Math.PI / 3}
      />
    </>
  );
}

/* ── Main Component ── */
export default function RouterView({ providers, models, apiHost, apiKey, isOnline }) {
  const [latest, setLatest] = useState({
    prompt: 'Querying DB indexes optimization',
    chosenProvider: 'subscription',
    chosenModel: 'claude-3-7-sonnet',
    complexity: 0.91,
    confidence: 0.96,
    reason: 'High coding complexity requested. Routed to premium subscription tier.',
    timestamp: new Date().toLocaleTimeString(),
    status: 200
  });

  const mockPrompts = useRef([
    { text: 'Optimizing high-throughput connection pools', provider: 'subscription', model: 'claude-3-7-sonnet', complexity: 0.94, confidence: 0.97, reason: 'High-reasoning prompt routed to Subscription model' },
    { text: 'Calculate average fibonacci sequence elements', provider: 'ollama', model: 'qwen3:latest', complexity: 0.12, confidence: 0.99, reason: 'Low complexity mathematical task routed to local Ollama instance' },
    { text: 'Drafting legal contract framework', provider: 'agy', model: 'gemini-3.1-pro', complexity: 0.78, confidence: 0.91, reason: 'Medium-high drafting routed to agy provider' },
    { text: 'Summarize transcript logs', provider: 'gemini', model: 'gemini-2.5-flash', complexity: 0.35, confidence: 0.88, reason: 'Summarization routed to Gemini' },
    { text: 'Generate SQL schema', provider: 'openai', model: 'gpt-4o', complexity: 0.68, confidence: 0.93, reason: 'Structured mapping routed to OpenAI' }
  ]);

  useEffect(() => {
    let iv;
    if (!isOnline) {
      iv = setInterval(() => {
        const rp = mockPrompts.current[Math.floor(Math.random() * mockPrompts.current.length)];
        setLatest({
          prompt: rp.text, chosenProvider: rp.provider,
          chosenModel: rp.model, complexity: rp.complexity,
          confidence: rp.confidence, reason: rp.reason,
          timestamp: new Date().toLocaleTimeString(), status: 200
        });
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
              const log = d.logs[0]; lastId = log.id;
              setLatest({
                prompt: log.prompt || (log.original_model === 'orchestrated' ? 'Orchestrated agent task' : 'Chat completions call'),
                chosenProvider: log.provider,
                chosenModel: log.model,
                complexity: log.complexity || 0.5,
                confidence: log.confidence || 1.0,
                reason: log.reason || 'Explicit route',
                timestamp: new Date(log.created_at).toLocaleTimeString(),
                status: log.status
              });
            }
          }
        } catch (e) {}
      }
      iv = setInterval(poll, 1500);
    }
    return () => clearInterval(iv);
  }, [isOnline, apiHost, apiKey]);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>

      {/* 3D Canvas */}
      <div className="three-container">
        <Canvas camera={{ position: [0, 1, 5], fov: 50 }} dpr={[1, 2]}>
          <RouterScene activeProvider={latest.chosenProvider} />
        </Canvas>
        <div className="three-overlay">
          <Sparkles size={10} style={{ color: 'var(--accent)' }} />
          LIVE 3D FLOW
        </div>
      </div>

      {/* Transaction Metadata */}
      <div className="card">
        <div className="card-header">Latest Transaction</div>
        <div style={{ display: 'grid', gridTemplateColumns: '1.4fr 1fr', gap: 16 }}>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
            <div>
              <div className="label" style={{ marginBottom: 6 }}>Prompt</div>
              <div className="code">"{latest.prompt}"</div>
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
              <div>
                <div className="label" style={{ marginBottom: 6 }}>Route</div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <ProviderLogo provider={latest.chosenProvider} size={18} />
                  <span style={{ fontWeight: 600 }}>{latest.chosenModel}</span>
                </div>
              </div>
              <div>
                <div className="label" style={{ marginBottom: 6 }}>Classification</div>
                <div style={{ display: 'flex', gap: 16, fontSize: 13 }}>
                  <span>
                    <span style={{ color: 'var(--text-muted)' }}>Cplx </span>
                    <span style={{ fontWeight: 700, color: 'var(--yellow)' }}>{(latest.complexity * 100).toFixed(0)}%</span>
                  </span>
                  <span>
                    <span style={{ color: 'var(--text-muted)' }}>Conf </span>
                    <span style={{ fontWeight: 700, color: 'var(--green)' }}>{(latest.confidence * 100).toFixed(0)}%</span>
                  </span>
                </div>
              </div>
            </div>
          </div>

          <div className="embed" style={{ borderLeftColor: latest.status === 200 ? 'var(--green)' : 'var(--red)' }}>
            <div className="label" style={{ marginBottom: 6 }}>Router Reasoning</div>
            <p style={{ fontSize: 13, color: 'var(--text-primary)', margin: 0, lineHeight: 1.6 }}>
              {latest.reason}
            </p>
            <div className="divider" />
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>{latest.timestamp}</span>
              <span className={`tag ${latest.status === 200 ? 'tag-success' : 'tag-error'}`}>
                HTTP {latest.status}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
