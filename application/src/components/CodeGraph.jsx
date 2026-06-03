import React, { useState, useEffect, useRef } from 'react';
import { motion } from 'framer-motion';
import { Network, RefreshCw, Search, ArrowRight, Check, AlertCircle, Sliders, Maximize2 } from 'lucide-react';

export default function CodeGraph({ apiHost, apiKey, isOnline }) {
  const [projects, setProjects] = useState([]);
  const [selectedProj, setSelectedProj] = useState('');
  const [graphData, setGraphData] = useState(null);
  const [loading, setLoading] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedNode, setSelectedNode] = useState(null);
  const [error, setError] = useState('');
  
  // Pathfinding states
  const [startNode, setStartNode] = useState('');
  const [endNode, setEndNode] = useState('');
  const [shortestPath, setShortestPath] = useState(null);
  const [pathfindingError, setPathfindingError] = useState('');

  // Physics states
  const [isPhysicsEnabled, setIsPhysicsEnabled] = useState(true);
  const [repelStr, setRepelStr] = useState(220);
  const [attractStr, setAttractStr] = useState(0.04);
  const [gravityStr, setGravityStr] = useState(0.015);

  const canvasRef = useRef(null);
  const simulationRef = useRef(null);

  // Viewport states for Zoom & Pan
  const transformRef = useRef({ x: 0, y: 0, k: 1 });
  const isDraggingRef = useRef(false);
  const dragStartRef = useRef({ x: 0, y: 0 });
  const draggedNodeRef = useRef(null);
  const wakeSimRef = useRef(null);

  // Node position persistence
  const nodesRef = useRef([]);

  // Drag detection refs
  const dragStartMouseRef = useRef({ x: 0, y: 0 });
  const hasDraggedRef = useRef(false);

  // Refs for canvas thread synchronization
  const searchQueryRef = useRef(searchQuery);
  const shortestPathRef = useRef(shortestPath);
  const selectedNodeRef = useRef(selectedNode);
  const isPhysicsEnabledRef = useRef(isPhysicsEnabled);
  const repelStrRef = useRef(repelStr);
  const attractStrRef = useRef(attractStr);
  const gravityStrRef = useRef(gravityStr);

  useEffect(() => { searchQueryRef.current = searchQuery; }, [searchQuery]);
  useEffect(() => { shortestPathRef.current = shortestPath; }, [shortestPath]);
  useEffect(() => { selectedNodeRef.current = selectedNode; }, [selectedNode]);
  useEffect(() => { isPhysicsEnabledRef.current = isPhysicsEnabled; }, [isPhysicsEnabled]);
  useEffect(() => { repelStrRef.current = repelStr; }, [repelStr]);
  useEffect(() => { attractStrRef.current = attractStr; }, [attractStr]);
  useEffect(() => { gravityStrRef.current = gravityStr; }, [gravityStr]);

  // Clear node cache when switching projects
  useEffect(() => {
    nodesRef.current = [];
  }, [selectedProj]);

  // Toggle physics and wake simulation
  const togglePhysics = () => {
    setIsPhysicsEnabled(prev => {
      const next = !prev;
      if (next && wakeSimRef.current) {
        wakeSimRef.current('', false);
      }
      return next;
    });
  };

  // Reset node coordinates to center with random offset
  const resetCoordinates = () => {
    if (nodesRef.current && canvasRef.current) {
      const canvas = canvasRef.current;
      nodesRef.current.forEach(n => {
        n.x = canvas.width / 2 + (Math.random() - 0.5) * 400;
        n.y = canvas.height / 2 + (Math.random() - 0.5) * 400;
        n.vx = 0;
        n.vy = 0;
        n.pinned = false;
      });
      if (wakeSimRef.current) {
        wakeSimRef.current('', false);
      }
    }
  };

  // Unpin all nodes
  const unpinAllNodes = () => {
    if (nodesRef.current) {
      nodesRef.current.forEach(n => {
        n.pinned = false;
      });
      if (wakeSimRef.current) {
        wakeSimRef.current('', false);
      }
    }
  };

  // Fit graph nodes to the viewport canvas boundaries
  const fitToFrame = () => {
    const canvas = canvasRef.current;
    if (!canvas || !nodesRef.current || nodesRef.current.length === 0) return;

    const nodes = nodesRef.current;
    let minX = Infinity, maxX = -Infinity;
    let minY = Infinity, maxY = -Infinity;

    nodes.forEach(n => {
      if (n.x < minX) minX = n.x;
      if (n.x > maxX) maxX = n.x;
      if (n.y < minY) minY = n.y;
      if (n.y > maxY) maxY = n.y;
    });

    const graphWidth = maxX - minX;
    const graphHeight = maxY - minY;

    // Fallback if width or height is negligible
    if (graphWidth < 5 || graphHeight < 5) {
      transformRef.current = {
        x: canvas.width / 2,
        y: canvas.height / 2,
        k: 0.5
      };
      return;
    }

    const padding = 60; // padding in pixels
    const canvasWidth = canvas.width;
    const canvasHeight = canvas.height;

    const scaleX = (canvasWidth - padding * 2) / graphWidth;
    const scaleY = (canvasHeight - padding * 2) / graphHeight;
    const nextK = Math.max(0.15, Math.min(1.5, Math.min(scaleX, scaleY)));

    const graphCenterX = minX + graphWidth / 2;
    const graphCenterY = minY + graphHeight / 2;

    transformRef.current = {
      x: canvasWidth / 2 - graphCenterX * nextK,
      y: canvasHeight / 2 - graphCenterY * nextK,
      k: nextK
    };
  };

  // Auto-fit to frame when graphData is loaded
  useEffect(() => {
    if (graphData && graphData.nodes && graphData.nodes.length > 0) {
      const timer = setTimeout(() => {
        fitToFrame();
      }, 350); // Delay slightly to allow nodes to disperse from center initial position
      return () => clearTimeout(timer);
    }
  }, [graphData]);

  // Fetch registered projects list
  useEffect(() => {
    async function loadProjects() {
      try {
        const res = await fetch(`${apiHost}/v1/projects`, {
          headers: { 'Authorization': `Bearer ${apiKey}` }
        });
        if (res.ok) {
          const data = await res.json();
          setProjects(data || []);
          if (data && data.length > 0) {
            setSelectedProj(data[0].path);
          }
        }
      } catch (err) {
        setError('Failed to fetch projects list.');
      }
    }
    loadProjects();
  }, [apiHost, apiKey]);

  // Fetch graph details for the selected project
  useEffect(() => {
    if (!selectedProj) return;
    async function loadGraph() {
      setLoading(true);
      setError('');
      setSelectedNode(null);
      setShortestPath(null);
      try {
        const res = await fetch(`${apiHost}/v1/graph?project_path=${encodeURIComponent(selectedProj)}`, {
          headers: { 'Authorization': `Bearer ${apiKey}` }
        });
        if (res.ok) {
          const data = await res.json();
          setGraphData(data);
        } else {
          setGraphData(null);
          setError('Graph data is not generated yet for this project. Try running a refresh.');
        }
      } catch (err) {
        setError('Connection failed. Could not load graph.');
      } finally {
        setLoading(false);
      }
    }
    loadGraph();
  }, [selectedProj, apiHost, apiKey]);

  // Run Canvas Force-Directed Layout Simulation
  useEffect(() => {
    if (!graphData || !graphData.nodes || graphData.nodes.length === 0) return;

    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');

    // Setup node position states (reuse existing positions if they are already in nodesRef)
    const existingNodesMap = new Map(nodesRef.current.map(n => [n.id, n]));
    const nodes = graphData.nodes.map(n => {
      const existing = existingNodesMap.get(n.id);
      if (existing) {
        return {
          ...n,
          x: existing.x,
          y: existing.y,
          vx: existing.vx,
          vy: existing.vy,
          pinned: existing.pinned,
          radius: n.file_type === 'file' ? 12 : 8
        };
      }
      return {
        ...n,
        x: canvas.width / 2 + (Math.random() - 0.5) * 400,
        y: canvas.height / 2 + (Math.random() - 0.5) * 400,
        vx: 0,
        vy: 0,
        radius: n.file_type === 'file' ? 12 : 8
      };
    });
    nodesRef.current = nodes;

    // Setup links index
    const links = graphData.links.map(l => {
      const sourceNode = nodes.find(n => n.id === l.source);
      const targetNode = nodes.find(n => n.id === l.target);
      return {
        ...l,
        sourceNode,
        targetNode
      };
    }).filter(l => l.sourceNode && l.targetNode);

    let animationFrameId;
    let hoveredNode = null;

    // Center viewport initially if nodesRef was empty
    if (existingNodesMap.size === 0) {
      transformRef.current = { x: canvas.width / 2, y: canvas.height / 2, k: 0.5 };
    }

    let alpha = 1.0;

    // Expose layout controller to the outer React context via ref
    wakeSimRef.current = (nodeId, pinStatus) => {
      alpha = 0.35; // Wake up physics
      if (nodeId) {
        const target = nodes.find(n => n.id === nodeId);
        if (target) {
          target.pinned = pinStatus;
        }
      }
    };

    const updatePhysics = () => {
      if (!isPhysicsEnabledRef.current) {
        return; // Physics is paused
      }

      if (alpha < 0.005) {
        return; // Physics cooled down and stabilized
      }

      const kAttract = attractStrRef.current * alpha;
      const kRepel = repelStrRef.current * alpha;
      const gravity = gravityStrRef.current * alpha;

      // Center of gravity force
      const cx = canvas.width / 2;
      const cy = canvas.height / 2;

      // 1. Repulsion between all pairs of nodes
      for (let i = 0; i < nodes.length; i++) {
        const n1 = nodes[i];
        for (let j = i + 1; j < nodes.length; j++) {
          const n2 = nodes[j];
          const dx = n2.x - n1.x;
          const dy = n2.y - n1.y;
          const distSq = dx * dx + dy * dy + 0.1;
          const dist = Math.sqrt(distSq);

          if (dist < 320) {
            const force = kRepel / distSq;
            const fx = (dx / dist) * force;
            const fy = (dy / dist) * force;

            n1.vx -= fx;
            n1.vy -= fy;
            n2.vx += fx;
            n2.vy += fy;
          }
        }

        // Pull to center
        n1.vx += (cx - n1.x) * gravity;
        n1.vy += (cy - n1.y) * gravity;
      }

      // 2. Attraction along edges
      for (let i = 0; i < links.length; i++) {
        const link = links[i];
        const n1 = link.sourceNode;
        const n2 = link.targetNode;
        const dx = n2.x - n1.x;
        const dy = n2.y - n1.y;
        const dist = Math.sqrt(dx * dx + dy * dy) + 0.1;
        
        const desiredDist = 90;
        const force = (dist - desiredDist) * kAttract;
        const fx = (dx / dist) * force;
        const fy = (dy / dist) * force;

        n1.vx += fx;
        n1.vy += fy;
        n2.vx -= fx;
        n2.vy -= fy;
      }

      // 3. Update positions with friction damping
      const damping = 0.85;
      for (let i = 0; i < nodes.length; i++) {
        const node = nodes[i];
        if (node === draggedNodeRef.current || node.pinned) {
          node.vx = 0;
          node.vy = 0;
          continue;
        }
        node.vx *= damping;
        node.vy *= damping;

        // Clamp maximum velocity to avoid wild swings
        const speed = Math.sqrt(node.vx * node.vx + node.vy * node.vy);
        if (speed > 10) {
          node.vx = (node.vx / speed) * 10;
          node.vy = (node.vy / speed) * 10;
        }

        node.x += node.vx;
        node.y += node.vy;
      }

      // Decay simulation temperature
      alpha *= 0.98;
    };

    const draw = () => {
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      ctx.save();

      // Apply pan/zoom transformations
      const { x, y, k } = transformRef.current;
      ctx.translate(x, y);
      ctx.scale(k, k);

      // Community color scheme matching auto-router premium palette
      const getCommunityColor = (comm, isDimmed) => {
        const colors = [
          '#6366f1', '#a855f7', '#ec4899', '#f43f5e', 
          '#10b981', '#06b6d4', '#3b82f6', '#f59e0b'
        ];
        const base = colors[comm % colors.length];
        return isDimmed ? `${base}25` : base;
      };

      const isPathLink = (link) => {
        const shortestPath = shortestPathRef.current;
        if (!shortestPath) return false;
        for (let i = 0; i < shortestPath.length - 1; i++) {
          const s = shortestPath[i];
          const t = shortestPath[i + 1];
          if ((link.source === s && link.target === t) || (link.source === t && link.target === s)) {
            return true;
          }
        }
        return false;
      };

      const isPathNode = (node) => {
        const shortestPath = shortestPathRef.current;
        if (!shortestPath) return false;
        return shortestPath.includes(node.id);
      };

      // 1. Draw Links
      ctx.lineWidth = 1.0;
      for (let i = 0; i < links.length; i++) {
        const l = links[i];
        const isPath = isPathLink(l);
        const isHighlighted = hoveredNode && (l.source === hoveredNode.id || l.target === hoveredNode.id);
        
        ctx.beginPath();
        ctx.moveTo(l.sourceNode.x, l.sourceNode.y);
        ctx.lineTo(l.targetNode.x, l.targetNode.y);
        
        if (isPath) {
          ctx.strokeStyle = 'var(--yellow)';
          ctx.lineWidth = 3.0;
        } else if (isHighlighted) {
          ctx.strokeStyle = 'rgba(255, 255, 255, 0.65)';
          ctx.lineWidth = 1.5;
        } else {
          ctx.strokeStyle = 'rgba(255, 255, 255, 0.08)';
          ctx.lineWidth = 1.0;
        }
        ctx.stroke();
      }

      // 2. Draw Nodes
      for (let i = 0; i < nodes.length; i++) {
        const n = nodes[i];
        const selectedNode = selectedNodeRef.current;
        const searchQuery = searchQueryRef.current;
        const shortestPath = shortestPathRef.current;

        const isSelected = selectedNode && selectedNode.id === n.id;
        const isHighlighted = hoveredNode && n.id === hoveredNode.id;
        const isNeighbor = hoveredNode && links.some(l => 
          (l.source === hoveredNode.id && l.target === n.id) || 
          (l.target === hoveredNode.id && l.source === n.id)
        );
        const matchesSearch = searchQuery && (
          n.id.toLowerCase().includes(searchQuery.toLowerCase()) ||
          n.label.toLowerCase().includes(searchQuery.toLowerCase())
        );

        const isPath = isPathNode(n);

        const isDimmed = (hoveredNode && !isHighlighted && !isNeighbor) || 
                         (searchQuery && !matchesSearch) || 
                         (shortestPath && !isPath);

        const color = getCommunityColor(n.community, isDimmed);

        ctx.beginPath();
        ctx.arc(n.x, n.y, n.radius + (isSelected ? 3 : 0), 0, 2 * Math.PI);
        ctx.fillStyle = color;
        ctx.fill();

        // Node Glowing Border
        if (isSelected) {
          ctx.strokeStyle = 'var(--accent)';
          ctx.lineWidth = 3.0;
          ctx.stroke();
        } else if (isHighlighted || isPath) {
          ctx.strokeStyle = isPath ? 'var(--yellow)' : '#fff';
          ctx.lineWidth = 2.0;
          ctx.stroke();
        } else if (matchesSearch) {
          ctx.strokeStyle = 'var(--green)';
          ctx.lineWidth = 2.5;
          ctx.stroke();
        }

        // Draw concentric dashed ring for pinned nodes
        if (n.pinned && !isDimmed) {
          ctx.beginPath();
          ctx.arc(n.x, n.y, n.radius + (isSelected ? 5 : 3), 0, 2 * Math.PI);
          ctx.strokeStyle = 'rgba(255, 255, 255, 0.45)';
          ctx.lineWidth = 1.0;
          ctx.setLineDash([2, 2]);
          ctx.stroke();
          ctx.setLineDash([]); // Reset dash
        }

        // Draw labels for large nodes or highlighted nodes
        const shouldShowLabel = k > 0.45 || isHighlighted || isSelected || isPath || matchesSearch;
        if (shouldShowLabel && !isDimmed) {
          ctx.fillStyle = 'rgba(255,255,255,0.85)';
          ctx.font = isSelected || isPath ? 'bold 11px Inter, sans-serif' : '10px Inter, sans-serif';
          ctx.textAlign = 'center';
          ctx.fillText(n.label, n.x, n.y - n.radius - 6);
        }
      }

      ctx.restore();
    };

    const loop = () => {
      updatePhysics();
      draw();
      animationFrameId = requestAnimationFrame(loop);
    };

    // Begin Animation Loop
    loop();

    // Mouse Interaction Handlers
    const getTransformedMouse = (clientX, clientY) => {
      const rect = canvas.getBoundingClientRect();
      const mx = clientX - rect.left;
      const my = clientY - rect.top;
      const { x, y, k } = transformRef.current;
      return {
        x: (mx - x) / k,
        y: (my - y) / k
      };
    };

    const findClosestNode = (mx, my) => {
      let closest = null;
      let minDist = 24; // trigger range threshold
      for (let i = 0; i < nodes.length; i++) {
        const n = nodes[i];
        const dx = n.x - mx;
        const dy = n.y - my;
        const dist = Math.sqrt(dx * dx + dy * dy);
        if (dist < minDist) {
          minDist = dist;
          closest = n;
        }
      }
      return closest;
    };

    const handleMouseDown = (e) => {
      const m = getTransformedMouse(e.clientX, e.clientY);
      const clicked = findClosestNode(m.x, m.y);

      if (clicked) {
        draggedNodeRef.current = clicked;
        dragStartMouseRef.current = { x: e.clientX, y: e.clientY };
        hasDraggedRef.current = false;
        alpha = 0.25; // Wake up physics
      } else {
        isDraggingRef.current = true;
        dragStartRef.current = { x: e.clientX - transformRef.current.x, y: e.clientY - transformRef.current.y };
      }
    };

    const handleMouseMove = (e) => {
      const m = getTransformedMouse(e.clientX, e.clientY);
      
      if (draggedNodeRef.current) {
        const dx = e.clientX - dragStartMouseRef.current.x;
        const dy = e.clientY - dragStartMouseRef.current.y;
        const dist = Math.sqrt(dx * dx + dy * dy);
        
        if (dist > 3) {
          hasDraggedRef.current = true;
          draggedNodeRef.current.pinned = true; // Pin on actual drag
        }

        draggedNodeRef.current.x = m.x;
        draggedNodeRef.current.y = m.y;
        alpha = Math.max(alpha, 0.2); // Keep physics awake while dragging
      } else if (isDraggingRef.current) {
        transformRef.current = {
          ...transformRef.current,
          x: e.clientX - dragStartRef.current.x,
          y: e.clientY - dragStartRef.current.y
        };
      } else {
        const closest = findClosestNode(m.x, m.y);
        if (closest !== hoveredNode) {
          hoveredNode = closest;
        }
      }
    };

    const handleMouseUp = (e) => {
      if (draggedNodeRef.current) {
        const node = draggedNodeRef.current;
        draggedNodeRef.current = null;
        
        if (!hasDraggedRef.current) {
          setSelectedNode(node);
        }
      }
      isDraggingRef.current = false;
    };

    const handleDoubleClick = (e) => {
      const m = getTransformedMouse(e.clientX, e.clientY);
      const clicked = findClosestNode(m.x, m.y);
      if (clicked) {
        clicked.pinned = !clicked.pinned;
        if (!clicked.pinned) {
          alpha = 0.25;
        }
        setSelectedNode(prev => prev && prev.id === clicked.id ? { ...clicked } : prev);
      }
    };

    const handleWheel = (e) => {
      e.preventDefault();
      const rect = canvas.getBoundingClientRect();
      const mx = e.clientX - rect.left;
      const my = e.clientY - rect.top;

      const { x, y, k } = transformRef.current;
      const factor = e.deltaY < 0 ? 1.15 : 0.85;
      const nextK = Math.max(0.1, Math.min(6, k * factor));

      // Zoom towards cursor location
      transformRef.current = {
        x: mx - (mx - x) * (nextK / k),
        y: my - (my - y) * (nextK / k),
        k: nextK
      };
    };

    // Bind event listeners
    canvas.addEventListener('mousedown', handleMouseDown);
    canvas.addEventListener('mousemove', handleMouseMove);
    canvas.addEventListener('mouseup', handleMouseUp);
    canvas.addEventListener('dblclick', handleDoubleClick);
    canvas.addEventListener('wheel', handleWheel);

    // Dynamic resize handler
    const resizeObserver = new ResizeObserver(entries => {
      for (let entry of entries) {
        canvas.width = entry.contentRect.width;
        canvas.height = entry.contentRect.height;
      }
    });
    resizeObserver.observe(canvas.parentElement);

    // Cleanup
    return () => {
      cancelAnimationFrame(animationFrameId);
      resizeObserver.disconnect();
      canvas.removeEventListener('mousedown', handleMouseDown);
      canvas.removeEventListener('mousemove', handleMouseMove);
      canvas.removeEventListener('mouseup', handleMouseUp);
      canvas.removeEventListener('dblclick', handleDoubleClick);
      canvas.removeEventListener('wheel', handleWheel);
    };
  }, [graphData]);

  // Trigger Graphify Update Incremental build
  const triggerRefresh = async () => {
    if (!selectedProj) return;
    setRefreshing(true);
    setError('');
    try {
      const res = await fetch(`${apiHost}/v1/graph/refresh?project_path=${encodeURIComponent(selectedProj)}`, {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${apiKey}` }
      });
      if (res.ok) {
        // Reload project graph data
        const graphRes = await fetch(`${apiHost}/v1/graph?project_path=${encodeURIComponent(selectedProj)}`, {
          headers: { 'Authorization': `Bearer ${apiKey}` }
        });
        if (graphRes.ok) {
          const data = await graphRes.json();
          setGraphData(data);
          setSelectedNode(null);
          setShortestPath(null);
        }
      } else {
        setError('Refresh execution failed.');
      }
    } catch (err) {
      setError('Connection failed. Could not refresh graph.');
    } finally {
      setRefreshing(false);
    }
  };

  // Find Path between start and end node
  const calculatePath = () => {
    if (!startNode || !endNode || !graphData) return;
    setPathfindingError('');
    setShortestPath(null);

    // Simple client-side BFS pathfinder on graphData
    const adj = {};
    graphData.links.forEach(l => {
      if (!adj[l.source]) adj[l.source] = [];
      adj[l.source].push(l.target);
    });

    const startExists = graphData.nodes.some(n => n.id === startNode);
    const endExists = graphData.nodes.some(n => n.id === endNode);

    if (!startExists || !endExists) {
      setPathfindingError('Start or end node ID does not exist in graph.');
      return;
    }

    const queue = [[startNode]];
    const visited = new Set([startNode]);

    while (queue.length > 0) {
      const path = queue.shift();
      const node = path[path.length - 1];

      if (node === endNode) {
        setShortestPath(path);
        return;
      }

      const neighbors = adj[node] || [];
      for (const neighbor of neighbors) {
        if (!visited.has(neighbor)) {
          visited.add(neighbor);
          queue.push([...path, neighbor]);
        }
      }
    }

    setPathfindingError('No dependency path found between these nodes.');
  };

  return (
    <div style={{ display: 'grid', gridTemplateColumns: '320px 1fr', gap: 16, height: 'calc(100vh - 120px)' }}>
      
      {/* ── Left Sidebar: Controls & Inspector ── */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 16, overflowY: 'auto', paddingRight: 4 }}>
        
        {/* Project Selector */}
        <div className="card">
          <div className="card-header" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <Network size={16} />
            <span>Workspace Codebase</span>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12, padding: '0 4px' }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
              <label style={{ fontSize: 11, color: 'var(--text-muted)', fontWeight: 600 }}>Active Project</label>
              <select 
                value={selectedProj}
                onChange={(e) => setSelectedProj(e.target.value)}
                style={{
                  background: 'var(--bg-dark)',
                  border: '1px solid var(--border)',
                  color: '#fff',
                  borderRadius: 6,
                  padding: 8,
                  fontSize: 12,
                  outline: 'none'
                }}
              >
                {projects.map(p => (
                  <option key={p.path} value={p.path}>
                    {p.path.split('/').pop()} ({p.nodes_count} nodes)
                  </option>
                ))}
                {projects.length === 0 && (
                  <option value="">No projects scanned</option>
                )}
              </select>
            </div>

            <button 
              onClick={triggerRefresh} 
              disabled={refreshing || !selectedProj}
              className="topbar-btn"
              style={{
                width: '100%',
                padding: '8px 12px',
                background: 'var(--accent)',
                color: '#fff',
                border: 'none',
                borderRadius: 6,
                fontWeight: 600,
                fontSize: 12,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                gap: 8,
                cursor: 'pointer',
                opacity: refreshing || !selectedProj ? 0.6 : 1
              }}
            >
              <RefreshCw size={14} className={refreshing ? 'spin' : ''} />
              {refreshing ? 'Rebuilding Graph...' : 'Refresh Code Graph'}
            </button>
          </div>
        </div>

        {/* Graph Simulation Controls */}
        <div className="card">
          <div className="card-header" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <Sliders size={16} />
            <span>Graph Physics & Layout</span>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 14, padding: '0 4px' }}>
            
            {/* Physics Active Toggle */}
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <label style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-secondary)' }}>Physics Simulation</label>
              <label className="toggle">
                <input 
                  type="checkbox" 
                  checked={isPhysicsEnabled}
                  onChange={togglePhysics}
                />
                <span className="track"></span>
                <span className="knob"></span>
              </label>
            </div>

            {/* Layout Actions */}
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 6 }}>
              <button 
                onClick={unpinAllNodes}
                title="Unpin all nodes"
                style={{
                  background: 'rgba(255,255,255,0.06)',
                  color: '#fff',
                  border: 'none',
                  borderRadius: 6,
                  padding: '6px 4px',
                  fontSize: 10,
                  fontWeight: 600,
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  gap: 3
                }}
              >
                📍 Unpin
              </button>
              <button 
                onClick={resetCoordinates}
                title="Reset layout coordinates"
                style={{
                  background: 'rgba(255,255,255,0.06)',
                  color: '#fff',
                  border: 'none',
                  borderRadius: 6,
                  padding: '6px 4px',
                  fontSize: 10,
                  fontWeight: 600,
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  gap: 3
                }}
              >
                🔄 Reset
              </button>
              <button 
                onClick={fitToFrame}
                title="Fit graph to viewport boundaries"
                style={{
                  background: 'rgba(255,255,255,0.06)',
                  color: '#fff',
                  border: 'none',
                  borderRadius: 6,
                  padding: '6px 4px',
                  fontSize: 10,
                  fontWeight: 600,
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  gap: 3
                }}
              >
                🔍 Fit
              </button>
            </div>

            <div style={{ height: '1px', background: 'rgba(255,255,255,0.04)' }} />

            {/* Physics Parameters */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 11 }}>
                  <span style={{ color: 'var(--text-muted)' }}>Repulsion (Node Push)</span>
                  <span style={{ color: 'var(--accent)', fontWeight: 600 }}>{repelStr}</span>
                </div>
                <input 
                  type="range" 
                  min="50" 
                  max="800" 
                  step="10"
                  value={repelStr}
                  onChange={(e) => setRepelStr(Number(e.target.value))}
                  style={{ width: '100%', accentColor: 'var(--accent)', background: 'rgba(255,255,255,0.05)', height: 4, borderRadius: 2 }}
                />
              </div>

              <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 11 }}>
                  <span style={{ color: 'var(--text-muted)' }}>Attraction (Edge Pull)</span>
                  <span style={{ color: 'var(--purple)', fontWeight: 600 }}>{attractStr}</span>
                </div>
                <input 
                  type="range" 
                  min="0.005" 
                  max="0.15" 
                  step="0.005"
                  value={attractStr}
                  onChange={(e) => setAttractStr(Number(e.target.value))}
                  style={{ width: '100%', accentColor: 'var(--purple)', background: 'rgba(255,255,255,0.05)', height: 4, borderRadius: 2 }}
                />
              </div>

              <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 11 }}>
                  <span style={{ color: 'var(--text-muted)' }}>Gravity (Pull to Center)</span>
                  <span style={{ color: 'var(--yellow)', fontWeight: 600 }}>{gravityStr}</span>
                </div>
                <input 
                  type="range" 
                  min="0.001" 
                  max="0.08" 
                  step="0.001"
                  value={gravityStr}
                  onChange={(e) => setGravityStr(Number(e.target.value))}
                  style={{ width: '100%', accentColor: 'var(--yellow)', background: 'rgba(255,255,255,0.05)', height: 4, borderRadius: 2 }}
                />
              </div>
            </div>

          </div>
        </div>

        {/* Pathfinder Panel */}
        <div className="card">
          <div className="card-header">Impact path finder</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10, padding: '0 4px' }}>
            <input 
              type="text" 
              placeholder="Start Node ID (e.g. mcp_main)"
              value={startNode}
              onChange={(e) => setStartNode(e.target.value)}
              className="search-input"
              style={{ padding: 8, fontSize: 12 }}
            />
            <input 
              type="text" 
              placeholder="End Node ID (e.g. db_go)"
              value={endNode}
              onChange={(e) => setEndNode(e.target.value)}
              className="search-input"
              style={{ padding: 8, fontSize: 12 }}
            />
            <button 
              onClick={calculatePath}
              className="topbar-btn"
              style={{
                background: 'var(--purple)',
                color: '#fff',
                padding: '8px 12px',
                border: 'none',
                borderRadius: 6,
                fontSize: 12,
                fontWeight: 600,
                cursor: 'pointer'
              }}
            >
              Trace Impact Path
            </button>
            {shortestPath && (
              <div style={{
                background: 'rgba(234, 179, 8, 0.1)',
                border: '1px solid var(--yellow)',
                borderRadius: 6,
                padding: 10,
                fontSize: 11
              }}>
                <div style={{ fontWeight: 700, color: 'var(--yellow)', marginBottom: 6 }}>Flow Route:</div>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4, alignItems: 'center' }}>
                  {shortestPath.map((p, idx) => (
                    <React.Fragment key={p}>
                      <span className="badge" style={{ background: 'rgba(255,255,255,0.06)', padding: '2px 6px' }}>{p}</span>
                      {idx < shortestPath.length - 1 && <ArrowRight size={10} style={{ color: 'var(--text-muted)' }} />}
                    </React.Fragment>
                  ))}
                </div>
                <button 
                  onClick={() => setShortestPath(null)}
                  style={{ background: 'none', border: 'none', color: 'var(--text-muted)', fontSize: 10, marginTop: 8, textDecoration: 'underline', cursor: 'pointer', padding: 0 }}
                >
                  Clear Path
                </button>
              </div>
            )}
            {pathfindingError && (
              <div style={{ color: 'var(--red)', fontSize: 11, display: 'flex', alignItems: 'center', gap: 4 }}>
                <AlertCircle size={12} />
                <span>{pathfindingError}</span>
              </div>
            )}
          </div>
        </div>

        {/* Selected Node Details (Inspector Panel) */}
        {selectedNode && (
          <div className="card" style={{ flexGrow: 1 }}>
            <div className="card-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span>Node Inspector</span>
              <button 
                onClick={() => setSelectedNode(null)}
                style={{ background: 'none', border: 'none', color: 'var(--text-muted)', fontSize: 11, cursor: 'pointer' }}
              >
                Close
              </button>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10, padding: '0 4px', fontSize: 12 }}>
              <div>
                <span style={{ color: 'var(--text-muted)', display: 'block', fontSize: 10, fontWeight: 700 }}>NODE ID</span>
                <span style={{ fontFamily: 'var(--font-mono)', wordBreak: 'break-all', fontWeight: 600 }}>{selectedNode.id}</span>
              </div>
              {selectedNode.label && (
                <div>
                  <span style={{ color: 'var(--text-muted)', display: 'block', fontSize: 10, fontWeight: 700 }}>LABEL</span>
                  <span style={{ fontWeight: 600, color: 'var(--accent)' }}>{selectedNode.label}</span>
                </div>
              )}
              {selectedNode.file_type && (
                <div>
                  <span style={{ color: 'var(--text-muted)', display: 'block', fontSize: 10, fontWeight: 700 }}>AST TYPE</span>
                  <span className="badge" style={{ fontSize: 10 }}>{selectedNode.file_type}</span>
                </div>
              )}
              {selectedNode.source_file && (
                <div>
                  <span style={{ color: 'var(--text-muted)', display: 'block', fontSize: 10, fontWeight: 700 }}>FILE PATH</span>
                  <span style={{ fontSize: 11 }}>{selectedNode.source_file}:{selectedNode.source_location}</span>
                </div>
              )}
              {selectedNode.description && (
                <div>
                  <span style={{ color: 'var(--text-muted)', display: 'block', fontSize: 10, fontWeight: 700 }}>SEMANTIC ROLE</span>
                  <p style={{ margin: 0, fontSize: 11, color: 'var(--text-dim)', lineHeight: 1.4 }}>{selectedNode.description}</p>
                </div>
              )}
              <div>
                <span style={{ color: 'var(--text-muted)', display: 'block', fontSize: 10, fontWeight: 700 }}>COMMUNITY CLUSTER</span>
                <span>Group #{selectedNode.community}</span>
              </div>
              <div>
                <span style={{ color: 'var(--text-muted)', display: 'block', fontSize: 10, fontWeight: 700 }}>LAYOUT POSITION</span>
                <button
                  onClick={() => {
                    const nextPinned = !selectedNode.pinned;
                    if (wakeSimRef.current) {
                      wakeSimRef.current(selectedNode.id, nextPinned);
                    }
                    setSelectedNode({ ...selectedNode, pinned: nextPinned });
                  }}
                  style={{
                    background: selectedNode.pinned ? 'var(--yellow)' : 'rgba(255,255,255,0.06)',
                    color: selectedNode.pinned ? '#000' : '#fff',
                    border: 'none',
                    borderRadius: 4,
                    padding: '4px 8px',
                    fontSize: 10,
                    fontWeight: 700,
                    cursor: 'pointer',
                    marginTop: 4,
                    display: 'flex',
                    alignItems: 'center',
                    gap: 4
                  }}
                >
                  {selectedNode.pinned ? '📌 Pinned (Click to Free)' : '📍 Free (Click to Pin)'}
                </button>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* ── Right Panel: Interactive Canvas Graph ── */}
      <div style={{ position: 'relative', background: 'var(--card-bg)', border: '1px solid var(--border)', borderRadius: 12, overflow: 'hidden' }}>
        
        {/* Search & Actions Bar */}
        <div style={{
          position: 'absolute',
          top: 12,
          left: 12,
          right: 12,
          zIndex: 10,
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          pointerEvents: 'none'
        }}>
          {/* Search bar input */}
          <div style={{ display: 'flex', alignItems: 'center', background: 'var(--bg-dark)', border: '1px solid var(--border)', borderRadius: 6, padding: '4px 10px', pointerEvents: 'auto', width: 260 }}>
            <Search size={14} style={{ color: 'var(--text-muted)', marginRight: 8 }} />
            <input 
              type="text" 
              placeholder="Search file or symbol..." 
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              style={{
                background: 'none',
                border: 'none',
                color: '#fff',
                fontSize: 12,
                outline: 'none',
                width: '100%'
              }}
            />
          </div>

          <div style={{
            background: 'var(--bg-dark)',
            border: '1px solid var(--border)',
            borderRadius: 6,
            padding: '6px 12px',
            fontSize: 11,
            color: 'var(--text-muted)'
          }}>
            Drag to pan | Scroll to zoom | Hover to trace connections
          </div>
        </div>

        {/* Loaders and Empty States */}
        {loading && (
          <div style={{ position: 'absolute', inset: 0, background: 'rgba(10,10,12,0.85)', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 16 }}>
            <RefreshCw size={24} className="spin" style={{ color: 'var(--accent)' }} />
            <span style={{ fontSize: 13, color: 'var(--text-muted)' }}>Loading codebase graph manifest...</span>
          </div>
        )}

        {error && (
          <div style={{ position: 'absolute', inset: 0, background: 'rgba(10,10,12,0.85)', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 12, padding: 32, textAlign: 'center' }}>
            <AlertCircle size={28} style={{ color: 'var(--red)' }} />
            <span style={{ fontSize: 13, color: '#fff', maxWidth: 400 }}>{error}</span>
          </div>
        )}

        {!graphData && !loading && !error && (
          <div style={{ position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 12 }}>
            <Network size={32} style={{ color: 'var(--text-muted)' }} />
            <span style={{ fontSize: 13, color: 'var(--text-muted)' }}>Please select a project to render graph visualization.</span>
          </div>
        )}

        {/* Canvas graph */}
        <canvas ref={canvasRef} style={{ width: '100%', height: '100%', display: 'block', cursor: 'grab' }} />

        {/* Floating Canvas Controls */}
        <div style={{
          position: 'absolute',
          bottom: 16,
          right: 16,
          zIndex: 10,
          display: 'flex',
          gap: 4,
          background: 'var(--bg-dark)',
          border: '1px solid var(--border)',
          borderRadius: 8,
          padding: 4,
          pointerEvents: 'auto'
        }}>
          <button 
            onClick={() => {
              const { x, y, k } = transformRef.current;
              const canvas = canvasRef.current;
              if (canvas) {
                const nextK = Math.min(6, k * 1.2);
                transformRef.current = {
                  x: canvas.width / 2 - (canvas.width / 2 - x) * (nextK / k),
                  y: canvas.height / 2 - (canvas.height / 2 - y) * (nextK / k),
                  k: nextK
                };
              }
            }}
            title="Zoom In"
            style={{
              width: 28,
              height: 28,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              background: 'none',
              border: 'none',
              color: 'var(--text-secondary)',
              cursor: 'pointer',
              fontSize: 16,
              fontWeight: 'bold',
              borderRadius: 6
            }}
            onMouseEnter={(e) => e.target.style.background = 'rgba(255,255,255,0.06)'}
            onMouseLeave={(e) => e.target.style.background = 'none'}
          >
            +
          </button>
          <button 
            onClick={() => {
              const { x, y, k } = transformRef.current;
              const canvas = canvasRef.current;
              if (canvas) {
                const nextK = Math.max(0.1, k / 1.2);
                transformRef.current = {
                  x: canvas.width / 2 - (canvas.width / 2 - x) * (nextK / k),
                  y: canvas.height / 2 - (canvas.height / 2 - y) * (nextK / k),
                  k: nextK
                };
              }
            }}
            title="Zoom Out"
            style={{
              width: 28,
              height: 28,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              background: 'none',
              border: 'none',
              color: 'var(--text-secondary)',
              cursor: 'pointer',
              fontSize: 16,
              fontWeight: 'bold',
              borderRadius: 6
            }}
            onMouseEnter={(e) => e.target.style.background = 'rgba(255,255,255,0.06)'}
            onMouseLeave={(e) => e.target.style.background = 'none'}
          >
            −
          </button>
          <div style={{ width: 1, background: 'var(--border)', margin: '4px 2px' }} />
          <button 
            onClick={fitToFrame}
            title="Fit to Screen"
            style={{
              width: 28,
              height: 28,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              background: 'none',
              border: 'none',
              color: 'var(--text-secondary)',
              cursor: 'pointer',
              borderRadius: 6
            }}
            onMouseEnter={(e) => e.target.style.background = 'rgba(255,255,255,0.06)'}
            onMouseLeave={(e) => e.target.style.background = 'none'}
          >
            <Maximize2 size={13} />
          </button>
        </div>
      </div>
    </div>
  );
}
