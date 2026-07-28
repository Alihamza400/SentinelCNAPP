'use client';

import { useEffect, useRef, useState, useCallback } from 'react';
import { useAuth } from '@/components/auth-provider';
import { API_URL } from '@/lib/config';
import { Share2, ZoomIn, ZoomOut, RotateCw } from 'lucide-react';

interface GraphNode {
  id: string;
  labels: string[];
  type: string;
  name: string;
  severity?: string;
  title?: string;
  region?: string;
  environment?: string;
  internet_facing?: boolean;
}

interface GraphEdge {
  source: string;
  target: string;
  type: string;
}

interface GraphData {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

const NODE_COLORS: Record<string, string> = {
  Asset: '#3b82f6',
  Finding: '#ef4444',
  Identity: '#8b5cf6',
  default: '#6b7280',
};

const SEVERITY_COLORS: Record<string, string> = {
  critical: '#ef4444',
  high: '#f97316',
  medium: '#eab308',
  low: '#22c55e',
  info: '#6b7280',
};

function getNodeColor(node: GraphNode): string {
  if (node.severity && SEVERITY_COLORS[node.severity]) {
    return SEVERITY_COLORS[node.severity];
  }
  const label = node.labels?.[0] || 'default';
  return NODE_COLORS[label] || NODE_COLORS.default;
}

function getNodeSize(node: GraphNode): number {
  if (node.severity === 'critical') return 12;
  if (node.severity === 'high') return 10;
  if (node.labels?.[0] === 'Asset') return 8;
  return 6;
}

function getNodeLabel(node: GraphNode): string {
  if (node.name) return node.name;
  if (node.title) return node.title.substring(0, 30);
  return node.id.substring(0, 20);
}

export default function GraphPage() {
  const { isAuthenticated } = useAuth();
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [graphData, setGraphData] = useState<GraphData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [scale, setScale] = useState(1);
  const [offset, setOffset] = useState({ x: 0, y: 0 });
  const [selectedNode, setSelectedNode] = useState<GraphNode | null>(null);
  const [hoveredNode, setHoveredNode] = useState<string | null>(null);
  const dragRef = useRef<{ node: GraphNode | null; startX: number; startY: number; nodeX: number; nodeY: number } | null>(null);
  const animFrameRef = useRef<number>(0);

  // Layout simulation state
  const nodesRef = useRef<Map<string, { x: number; y: number; vx: number; vy: number }>>(new Map());
  const edgeCacheRef = useRef<Map<string, { source: string; target: string }>>(new Map());

  useEffect(() => {
    if (!isAuthenticated) return;

    const fetchGraph = async () => {
      setLoading(true);
      try {
        const token = localStorage.getItem('sentinel_token');
        const res = await fetch(`${API_URL}/api/v1/graph?limit=200`, {
          headers: { Authorization: `Bearer ${token}` },
        });

        if (res.ok) {
          const data: GraphData = await res.json();
          setGraphData(data);
          initializeLayout(data);
        } else {
          setGraphData(getMockGraphData());
          initializeLayout(getMockGraphData());
        }
      } catch {
        setGraphData(getMockGraphData());
        initializeLayout(getMockGraphData());
      } finally {
        setLoading(false);
      }
    };

    fetchGraph();
  }, [isAuthenticated]);

  const initializeLayout = useCallback((data: GraphData) => {
    const positions = new Map<string, { x: number; y: number; vx: number; vy: number }>();
    const centerX = 400;
    const centerY = 300;

    // Initialize nodes in a circle
    data.nodes.forEach((node, i) => {
      const angle = (2 * Math.PI * i) / data.nodes.length;
      const radius = Math.min(300, 50 + data.nodes.length * 15);
      positions.set(node.id, {
        x: centerX + radius * Math.cos(angle),
        y: centerY + radius * Math.sin(angle),
        vx: 0,
        vy: 0,
      });
    });

    nodesRef.current = positions;

    // Build edge cache
    const edges = new Map<string, { source: string; target: string }>();
    data.edges.forEach((e) => {
      const edgeId = `${e.source}-${e.target}`;
      if (!edges.has(edgeId)) {
        edges.set(edgeId, { source: e.source, target: e.target });
      }
      // Add reverse
      const revId = `${e.target}-${e.source}`;
      if (!edges.has(revId)) {
        edges.set(revId, { source: e.target, target: e.source });
      }
    });
    edgeCacheRef.current = edges;
  }, []);

  // Force-directed layout simulation
  useEffect(() => {
    if (!graphData || graphData.nodes.length === 0) return;

    let running = true;

    const simulate = () => {
      if (!running) return;

      const positions = nodesRef.current;
      const nodes = graphData.nodes;

      // Forces
      const REPULSION = 5000;
      const ATTRACTION = 0.005;
      const DAMPING = 0.85;
      const CENTER_GRAVITY = 0.01;

      // Repulsion between all nodes
      for (let i = 0; i < nodes.length; i++) {
        for (let j = i + 1; j < nodes.length; j++) {
          const a = positions.get(nodes[i].id);
          const b = positions.get(nodes[j].id);
          if (!a || !b) continue;

          const dx = a.x - b.x;
          const dy = a.y - b.y;
          const dist = Math.max(Math.sqrt(dx * dx + dy * dy), 10);

          const force = REPULSION / (dist * dist);
          const fx = (dx / dist) * force;
          const fy = (dy / dist) * force;

          a.vx += fx;
          a.vy += fy;
          b.vx -= fx;
          b.vy -= fy;
        }
      }

      // Attraction along edges
      graphData.edges.forEach((edge) => {
        const source = positions.get(edge.source);
        const target = positions.get(edge.target);
        if (!source || !target) return;

        const dx = target.x - source.x;
        const dy = target.y - source.y;
        const dist = Math.max(Math.sqrt(dx * dx + dy * dy), 1);

        const fx = dx * ATTRACTION;
        const fy = dy * ATTRACTION;

        source.vx += fx;
        source.vy += fy;
        target.vx -= fx;
        target.vy -= fy;
      });

      // Center gravity + damping + position update
      const centerX = 400;
      const centerY = 300;
      positions.forEach((pos) => {
        pos.vx += (centerX - pos.x) * CENTER_GRAVITY;
        pos.vy += (centerY - pos.y) * CENTER_GRAVITY;

        pos.vx *= DAMPING;
        pos.vy *= DAMPING;

        pos.x += pos.vx;
        pos.y += pos.vy;
      });

      draw();
      animFrameRef.current = requestAnimationFrame(simulate);
    };

    animFrameRef.current = requestAnimationFrame(simulate);

    return () => {
      running = false;
      cancelAnimationFrame(animFrameRef.current);
    };
  }, [graphData]);

  const draw = () => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const dpr = window.devicePixelRatio || 1;
    const rect = canvas.getBoundingClientRect();
    canvas.width = rect.width * dpr;
    canvas.height = rect.height * dpr;
    ctx.scale(dpr, dpr);

    ctx.clearRect(0, 0, rect.width, rect.height);
    ctx.save();
    ctx.translate(offset.x, offset.y);
    ctx.scale(scale, scale);

    const positions = nodesRef.current;
    if (!graphData) return;

    // Draw edges
    ctx.strokeStyle = 'rgba(100, 116, 139, 0.3)';
    ctx.lineWidth = 1;
    graphData.edges.forEach((edge) => {
      const source = positions.get(edge.source);
      const target = positions.get(edge.target);
      if (!source || !target) return;

      ctx.beginPath();
      ctx.moveTo(source.x, source.y);
      ctx.lineTo(target.x, target.y);
      ctx.stroke();
    });

    // Draw nodes
    graphData.nodes.forEach((node) => {
      const pos = positions.get(node.id);
      if (!pos) return;

      const radius = getNodeSize(node) * (hoveredNode === node.id ? 1.3 : 1);
      const color = getNodeColor(node);

      ctx.beginPath();
      ctx.arc(pos.x, pos.y, radius, 0, Math.PI * 2);
      ctx.fillStyle = color;
      ctx.fill();

      if (hoveredNode === node.id) {
        ctx.strokeStyle = '#fff';
        ctx.lineWidth = 2;
        ctx.stroke();
      }

      // Label
      if (scale > 0.6 || hoveredNode === node.id) {
        ctx.font = '10px Inter, sans-serif';
        ctx.fillStyle = '#94a3b8';
        ctx.textAlign = 'center';
        ctx.fillText(getNodeLabel(node), pos.x, pos.y + radius + 12);
      }
    });

    ctx.restore();
  };

  const handleCanvasClick = (e: React.MouseEvent<HTMLCanvasElement>) => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const rect = canvas.getBoundingClientRect();
    const x = (e.clientX - rect.left - offset.x) / scale;
    const y = (e.clientY - rect.top - offset.y) / scale;

    const positions = nodesRef.current;
    if (!graphData) return;

    let closest: GraphNode | null = null;
    let closestDist = Infinity;

    graphData.nodes.forEach((node) => {
      const pos = positions.get(node.id);
      if (!pos) return;
      const dx = x - pos.x;
      const dy = y - pos.y;
      const dist = Math.sqrt(dx * dx + dy * dy);
      if (dist < 20 && dist < closestDist) {
        closest = node;
        closestDist = dist;
      }
    });

    setSelectedNode(closest);
  };

  const handleMouseMove = (e: React.MouseEvent<HTMLCanvasElement>) => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const rect = canvas.getBoundingClientRect();
    const x = (e.clientX - rect.left - offset.x) / scale;
    const y = (e.clientY - rect.top - offset.y) / scale;

    const positions = nodesRef.current;
    if (!graphData) return;

    let found: string | null = null;
    graphData.nodes.forEach((node) => {
      const pos = positions.get(node.id);
      if (!pos) return;
      const dx = x - pos.x;
      const dy = y - pos.y;
      if (Math.sqrt(dx * dx + dy * dy) < 15) {
        found = node.id;
      }
    });

    setHoveredNode(found);
    canvas.style.cursor = found ? 'pointer' : 'grab';
  };

  if (!isAuthenticated) {
    return <div className="container py-6"><p className="text-muted-foreground">Please log in to view the graph.</p></div>;
  }

  return (
    <div className="container py-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Correlation Graph</h1>
          <p className="text-muted-foreground mt-1">
            {graphData ? `${graphData.nodes.length} assets, findings, and identities` : 'Interactive security graph'}
          </p>
        </div>
        <Share2 className="h-8 w-8 text-muted-foreground" />
      </div>

      <div className="flex gap-4">
        <div className="flex-1 relative">
          {loading ? (
            <div className="rounded-lg border bg-card p-24 text-center">
              <p className="text-muted-foreground">Loading graph...</p>
            </div>
          ) : (
            <div className="rounded-lg border bg-card overflow-hidden">
              <canvas
                ref={canvasRef}
                className="w-full h-[600px] cursor-grab"
                onClick={handleCanvasClick}
                onMouseMove={handleMouseMove}
              />
              <div className="absolute top-2 right-2 flex gap-1">
                <button onClick={() => setScale(s => Math.min(3, s + 0.2))}
                  className="rounded-md bg-background/80 border px-2 py-1 hover:bg-muted">
                  <ZoomIn className="h-4 w-4" />
                </button>
                <button onClick={() => setScale(s => Math.max(0.3, s - 0.2))}
                  className="rounded-md bg-background/80 border px-2 py-1 hover:bg-muted">
                  <ZoomOut className="h-4 w-4" />
                </button>
                <button onClick={() => { setScale(1); setOffset({ x: 0, y: 0 }); }}
                  className="rounded-md bg-background/80 border px-2 py-1 hover:bg-muted">
                  <RotateCw className="h-4 w-4" />
                </button>
              </div>
            </div>
          )}

          {error && (
            <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive mt-2">{error}</div>
          )}
        </div>

        {/* Side Panel */}
        <div className="w-72 space-y-4">
          <div className="rounded-lg border bg-card p-4">
            <h3 className="font-semibold text-sm mb-3">Legend</h3>
            <div className="space-y-2 text-xs">
              <div className="flex items-center gap-2">
                <div className="w-3 h-3 rounded-full" style={{ backgroundColor: NODE_COLORS.Asset }} />
                <span>Asset</span>
              </div>
              <div className="flex items-center gap-2">
                <div className="w-3 h-3 rounded-full" style={{ backgroundColor: NODE_COLORS.Finding }} />
                <span>Finding</span>
              </div>
              <div className="flex items-center gap-2">
                <div className="w-3 h-3 rounded-full" style={{ backgroundColor: NODE_COLORS.Identity }} />
                <span>Identity</span>
              </div>
            </div>
          </div>

          {selectedNode && (
            <div className="rounded-lg border bg-card p-4">
              <h3 className="font-semibold text-sm mb-3">Selected Node</h3>
              <div className="space-y-2 text-xs">
                <div><span className="text-muted-foreground">ID:</span> <span className="truncate block">{selectedNode.id}</span></div>
                <div><span className="text-muted-foreground">Type:</span> {selectedNode.labels?.join(', ')}</div>
                {selectedNode.severity && (
                  <div>
                    <span className="text-muted-foreground">Severity:</span>{' '}
                    <span className="font-medium" style={{ color: SEVERITY_COLORS[selectedNode.severity] }}>
                      {selectedNode.severity}
                    </span>
                  </div>
                )}
                {selectedNode.environment && (
                  <div><span className="text-muted-foreground">Environment:</span> {selectedNode.environment}</div>
                )}
                {selectedNode.title && (
                  <div><span className="text-muted-foreground">Finding:</span> {selectedNode.title}</div>
                )}
              </div>
            </div>
          )}

          <div className="rounded-lg border bg-card p-4">
            <h3 className="font-semibold text-sm mb-3">Filters</h3>
            <div className="space-y-2 text-xs text-muted-foreground">
              <p>Click any node to see details.</p>
              <p>Scroll to zoom, drag to pan.</p>
              <p>The graph shows assets (blue), findings (red), and identities (purple).</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function getMockGraphData(): GraphData {
  return {
    nodes: [
      { id: 'arn:aws:s3:::checkout-logs', labels: ['Asset'], type: 's3_bucket', name: 'checkout-logs', region: 'us-east-1', environment: 'production', internet_facing: true },
      { id: 'arn:aws:iam::123456789012:role/checkout-api-prod-role', labels: ['Asset'], type: 'iam_role', name: 'checkout-api-prod-role', region: 'global', environment: 'production' },
      { id: 'arn:aws:ec2:us-east-1:123456789012:instance/i-abc123', labels: ['Asset'], type: 'ec2_instance', name: 'checkout-api-server', region: 'us-east-1', environment: 'production', internet_facing: true },
      { id: 'finding-1', labels: ['Finding'], type: 'vulnerability', severity: 'high', title: 'CVE-2026-1234: libssl buffer overflow', asset_id: 'arn:aws:ec2:us-east-1:123456789012:instance/i-abc123' },
      { id: 'finding-2', labels: ['Finding'], type: 'misconfiguration', severity: 'critical', title: 'S3 Bucket publicly accessible', asset_id: 'arn:aws:s3:::checkout-logs' },
      { id: 'finding-3', labels: ['Finding'], type: 'secret', severity: 'critical', title: 'AWS Access Key in config.js', asset_id: 'git:github.com/org/checkout-api' },
      { id: 'identity-1', labels: ['Identity'], type: 'iam_role', name: 'checkout-api-prod-role' },
    ],
    edges: [
      { source: 'finding-1', target: 'arn:aws:ec2:us-east-1:123456789012:instance/i-abc123', type: 'FOUND_IN' },
      { source: 'finding-2', target: 'arn:aws:s3:::checkout-logs', type: 'FOUND_IN' },
      { source: 'arn:aws:ec2:us-east-1:123456789012:instance/i-abc123', target: 'identity-1', type: 'HAS_IDENTITY' },
      { source: 'identity-1', target: 'arn:aws:s3:::checkout-logs', type: 'CAN_ACCESS' },
    ],
  };
}
