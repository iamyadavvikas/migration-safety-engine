import { useEffect, useRef, useMemo } from 'react'
import { select } from 'd3-selection'
import type { State } from '../types'
import { STATE_COLORS, STATE_LABELS, STATE_FLOW } from '../types'

interface Props {
  currentState: State
}

interface Link {
  source: string
  target: string
}

export default function StateMachineGraph({ currentState }: Props) {
  const svgRef = useRef<SVGSVGElement>(null)

  const nodes = useMemo(() => {
    const nodeW = 100
    const nodeH = 40
    const gap = 10
    const height = 160
    return STATE_FLOW.map((s, i) => ({
      id: s,
      label: STATE_LABELS[s],
      state: s,
      x: 30 + i * (nodeW + gap),
      y: height / 2 - nodeH / 2,
    }))
  }, [])

  const links = useMemo(() => {
    const result: Link[] = []
    for (let i = 0; i < nodes.length - 1; i++) {
      result.push({ source: nodes[i].id, target: nodes[i + 1].id })
    }
    return result
  }, [nodes])

  useEffect(() => {
    if (!svgRef.current) return

    const svg = select(svgRef.current)
    svg.selectAll('*').remove()

    const nodeW = 100
    const nodeH = 40
    const currentIdx = STATE_FLOW.indexOf(currentState)
    const isTerminal = currentState === 'Done' || currentState === 'RolledBack'
    const isSuccess = currentState === 'Done'

    // Defs
    const defs = svg.append('defs')

    // Glow filter
    const glow = defs.append('filter')
      .attr('id', 'sm-glow')
      .attr('x', '-50%').attr('y', '-50%')
      .attr('width', '200%').attr('height', '200%')
    glow.append('feGaussianBlur').attr('stdDeviation', '4').attr('result', 'coloredBlur')
    const feMerge = glow.append('feMerge')
    feMerge.append('feMergeNode').attr('in', 'coloredBlur')
    feMerge.append('feMergeNode').attr('in', 'SourceGraphic')

    // Strong glow
    const glowSuccess = defs.append('filter')
      .attr('id', 'sm-glow-strong')
      .attr('x', '-50%').attr('y', '-50%')
      .attr('width', '200%').attr('height', '200%')
    glowSuccess.append('feGaussianBlur').attr('stdDeviation', '6').attr('result', 'coloredBlur')
    const feMerge2 = glowSuccess.append('feMerge')
    feMerge2.append('feMergeNode').attr('in', 'coloredBlur')
    feMerge2.append('feMergeNode').attr('in', 'SourceGraphic')

    // Arrowhead
    defs.append('marker')
      .attr('id', 'sm-arrow')
      .attr('viewBox', '0 0 10 10')
      .attr('refX', 9).attr('refY', 5)
      .attr('markerWidth', 6).attr('markerHeight', 6)
      .attr('orient', 'auto')
      .append('path').attr('d', 'M 0 0 L 10 5 L 0 10 z')
      .attr('fill', '#4a5568')

    defs.append('marker')
      .attr('id', 'sm-arrow-active')
      .attr('viewBox', '0 0 10 10')
      .attr('refX', 9).attr('refY', 5)
      .attr('markerWidth', 6).attr('markerHeight', 6)
      .attr('orient', 'auto')
      .append('path').attr('d', 'M 0 0 L 10 5 L 0 10 z')
      .attr('fill', STATE_COLORS[currentState])

    const g = svg.append('g').attr('transform', 'translate(0, 10)')

    // Links
    links.forEach((link) => {
      const src = nodes.find(n => n.id === link.source)
      const tgt = nodes.find(n => n.id === link.target)
      if (!src || !tgt) return
      
      const srcIdx = STATE_FLOW.indexOf(src.state)
      const isActive = srcIdx < currentIdx

      g.append('line')
        .attr('x1', src.x + nodeW).attr('y1', src.y + nodeH / 2)
        .attr('x2', tgt.x).attr('y2', tgt.y + nodeH / 2)
        .attr('stroke', isActive ? STATE_COLORS[src.state] : '#2a3050')
        .attr('stroke-width', isActive ? 2 : 1)
        .attr('stroke-opacity', isActive ? 0.6 : 0.3)
        .attr('marker-end', isActive ? 'url(#sm-arrow-active)' : 'url(#sm-arrow)')

      if (isActive) {
        const flowLine = g.append('line')
          .attr('x1', src.x + nodeW).attr('y1', src.y + nodeH / 2)
          .attr('x2', tgt.x).attr('y2', tgt.y + nodeH / 2)
          .attr('stroke', STATE_COLORS[src.state])
          .attr('stroke-width', 2).attr('stroke-opacity', 0.8)
          .attr('stroke-dasharray', '4,8')
          .attr('stroke-dashoffset', '0')
        flowLine.append('animate')
          .attr('attributeName', 'stroke-dashoffset')
          .attr('from', '12').attr('to', '0')
          .attr('dur', '1s').attr('repeatCount', 'indefinite')
      }
    })

    // Nodes
    const gNodes = g.selectAll('g.node')
      .data(nodes)
      .join('g')
      .attr('class', 'node')
      .attr('transform', d => `translate(${d.x}, ${d.y})`)

    gNodes.append('rect')
      .attr('width', nodeW).attr('height', nodeH)
      .attr('rx', 10)
      .attr('fill', d => {
        const idx = STATE_FLOW.indexOf(d.state)
        if (idx < currentIdx) return `${STATE_COLORS[d.state]}30`
        if (idx === currentIdx) return `${STATE_COLORS[d.state]}20`
        return 'rgba(15, 20, 40, 0.8)'
      })
      .attr('stroke', d => {
        const idx = STATE_FLOW.indexOf(d.state)
        if (d.state === currentState && !isTerminal) return STATE_COLORS[d.state]
        if (idx < currentIdx) return `${STATE_COLORS[d.state]}60`
        if (idx === currentIdx && isTerminal) return STATE_COLORS[d.state]
        return 'rgba(255, 255, 255, 0.08)'
      })
      .attr('stroke-width', d => (d.state === currentState) ? 2 : 1)
      .attr('filter', d => {
        if (d.state === currentState && isSuccess) return 'url(#sm-glow-strong)'
        if (d.state === currentState && !isTerminal) return 'url(#sm-glow)'
        return 'none'
      })

    gNodes.append('text')
      .attr('x', nodeW / 2).attr('y', nodeH / 2)
      .attr('dy', '0.35em').attr('text-anchor', 'middle')
      .attr('fill', d => STATE_FLOW.indexOf(d.state) <= currentIdx ? '#f0f2f8' : '#5a6478')
      .attr('font-size', '11px')
      .attr('font-weight', d => d.state === currentState ? '700' : '500')
      .attr('font-family', 'Inter, sans-serif')
      .text(d => d.label)

    // Current state ring
    if (currentIdx >= 0 && !isTerminal) {
      const cur = nodes[currentIdx]
      if (cur) {
        const ring = g.append('rect')
          .attr('x', cur.x - 5).attr('y', cur.y - 5)
          .attr('width', nodeW + 10).attr('height', nodeH + 10)
          .attr('rx', 13).attr('fill', 'none')
          .attr('stroke', STATE_COLORS[currentState])
          .attr('stroke-width', 2).attr('stroke-dasharray', '6,4').attr('opacity', 0.8)
        ring.append('animate')
          .attr('attributeName', 'stroke-dashoffset')
          .attr('from', '0').attr('to', '20')
          .attr('dur', '1.5s').attr('repeatCount', 'indefinite')

        // Pulsing outer glow
        g.append('rect')
          .attr('x', cur.x - 8).attr('y', cur.y - 8)
          .attr('width', nodeW + 16).attr('height', nodeH + 16)
          .attr('rx', 16).attr('fill', 'none')
          .attr('stroke', STATE_COLORS[currentState])
          .attr('stroke-width', 1).attr('opacity', 0.3)
          .append('animate')
          .attr('attributeName', 'opacity')
          .attr('values', '0.3;0.1;0.3')
          .attr('dur', '2s').attr('repeatCount', 'indefinite')
      }
    }

    // Terminal effects
    if (isTerminal && currentIdx >= 0) {
      const cur = nodes[currentIdx]
      if (cur) {
        g.append('rect')
          .attr('x', cur.x - 6).attr('y', cur.y - 6)
          .attr('width', nodeW + 12).attr('height', nodeH + 12)
          .attr('rx', 14).attr('fill', 'none')
          .attr('stroke', STATE_COLORS[currentState])
          .attr('stroke-width', 2)
          .attr('stroke-dasharray', isSuccess ? 'none' : '4,3')
          .attr('opacity', 0.6)
      }
    }

  }, [currentState, nodes, links])

  return (
    <div className="state-machine-container fade-in">
      <div style={{
        padding: '14px 20px',
        borderBottom: '1px solid var(--border)',
        fontSize: '0.85rem',
        fontWeight: 600,
        display: 'flex',
        alignItems: 'center',
        gap: 8,
      }}>
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
          <circle cx="3" cy="8" r="2"/><circle cx="8" cy="3" r="2"/><circle cx="13" cy="8" r="2"/>
          <path d="M5 8h2M10 3v2M10 8h1"/>
        </svg>
        State Machine Flow
      </div>
      <div style={{ overflowX: 'auto', padding: '8px 0' }}>
        <svg
          ref={svgRef}
          width="100%"
          height={160}
          viewBox="0 0 920 160"
          preserveAspectRatio="xMidYMid meet"
          role="img"
          aria-label={`State machine showing migration in state: ${STATE_LABELS[currentState]}. Flow: ${STATE_FLOW.map(s => STATE_LABELS[s]).join(' → ')}`}
          style={{ display: 'block', minWidth: 600 }}
        >
          <title>Migration State Machine — Current: {STATE_LABELS[currentState]}</title>
        </svg>
      </div>
    </div>
  )
}
