import { useEffect, useRef } from 'react'
import * as d3 from 'd3'
import type { State } from '../types'
import { STATE_COLORS, STATE_LABELS, STATE_FLOW } from '../types'

interface Props {
  currentState: State
}

interface Node {
  id: string
  label: string
  state: State
  x: number
  y: number
}

interface Link {
  source: string
  target: string
}

export default function StateMachineGraph({ currentState }: Props) {
  const svgRef = useRef<SVGSVGElement>(null)

  useEffect(() => {
    if (!svgRef.current) return

    const svg = d3.select(svgRef.current)
    svg.selectAll('*').remove()

    const height = 160
    const nodeW = 100
    const nodeH = 40
    const gap = 10

    const nodes: Node[] = STATE_FLOW.map((s, i) => ({
      id: s,
      label: STATE_LABELS[s],
      state: s,
      x: 30 + i * (nodeW + gap),
      y: height / 2 - nodeH / 2,
    }))

    const links: Link[] = []
    for (let i = 0; i < nodes.length - 1; i++) {
      links.push({ source: nodes[i].id, target: nodes[i + 1].id })
    }

    const currentIdx = STATE_FLOW.indexOf(currentState)
    const isTerminal = currentState === 'Done' || currentState === 'RolledBack'

    const g = svg.append('g').attr('transform', `translate(0, 10)`)

    g.selectAll('line')
      .data(links)
      .join('line')
      .attr('x1', d => nodes.find(n => n.id === d.source)!.x + nodeW)
      .attr('y1', d => nodes.find(n => n.id === d.source)!.y + nodeH / 2)
      .attr('x2', d => nodes.find(n => n.id === d.target)!.x)
      .attr('y2', d => nodes.find(n => n.id === d.target)!.y + nodeH / 2)
      .attr('stroke', d => {
        const src = nodes.find(n => n.id === d.source)!
        return STATE_COLORS[src.state]
      })
      .attr('stroke-width', 2)
      .attr('stroke-opacity', 0.4)
      .attr('marker-end', 'url(#arrowhead)')

    svg.append('defs').append('marker')
      .attr('id', 'arrowhead')
      .attr('viewBox', '0 0 10 10')
      .attr('refX', 10)
      .attr('refY', 5)
      .attr('markerWidth', 6)
      .attr('markerHeight', 6)
      .attr('orient', 'auto')
      .append('path')
      .attr('d', 'M 0 0 L 10 5 L 0 10 z')
      .attr('fill', 'var(--text-muted)')

    const gNodes = g.selectAll('g.node')
      .data(nodes)
      .join('g')
      .attr('class', 'node')
      .attr('transform', d => `translate(${d.x}, ${d.y})`)

    gNodes.append('rect')
      .attr('width', nodeW)
      .attr('height', nodeH)
      .attr('rx', 8)
      .attr('fill', d => {
        const idx = STATE_FLOW.indexOf(d.state)
        if (idx < currentIdx) return `${STATE_COLORS[d.state]}44`
        if (idx === currentIdx) return STATE_COLORS[d.state]
        return 'var(--bg-card)'
      })
      .attr('stroke', d => {
        if (d.state === currentState) return STATE_COLORS[d.state]
        return 'var(--border)'
      })
      .attr('stroke-width', d => d.state === currentState ? 3 : 1)

    gNodes.append('text')
      .attr('x', nodeW / 2)
      .attr('y', nodeH / 2)
      .attr('dy', '0.35em')
      .attr('text-anchor', 'middle')
      .attr('fill', d => {
        const idx = STATE_FLOW.indexOf(d.state)
        if (idx <= currentIdx) return '#fff'
        return 'var(--text-muted)'
      })
      .attr('font-size', '11px')
      .attr('font-weight', d => d.state === currentState ? '700' : '500')
      .text(d => d.label)

    // Current state indicator ring
    if (currentIdx >= 0 && !isTerminal) {
      const cur = nodes[currentIdx]
      g.append('rect')
        .attr('x', cur.x - 4)
        .attr('y', cur.y - 4)
        .attr('width', nodeW + 8)
        .attr('height', nodeH + 8)
        .attr('rx', 12)
        .attr('fill', 'none')
        .attr('stroke', STATE_COLORS[currentState])
        .attr('stroke-width', 2)
        .attr('stroke-dasharray', '4,3')
        .attr('opacity', 0.7)
        .append('animate')
        .attr('attributeName', 'stroke-dashoffset')
        .attr('from', '0')
        .attr('to', '14')
        .attr('dur', '1s')
        .attr('repeatCount', 'indefinite')
    }

  }, [currentState])

  return (
    <div className="state-machine-container">
      <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--border)', fontSize: '0.9rem', fontWeight: 600 }}>
        State Machine Flow
      </div>
      <div style={{ overflowX: 'auto', padding: '8px 0' }}>
        <svg ref={svgRef} width={900} height={160} style={{ display: 'block', margin: '0 auto', minWidth: 850 }} />
      </div>
    </div>
  )
}
