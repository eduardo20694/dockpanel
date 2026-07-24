import { useEffect, useState } from 'react'
import { useTheme } from '../context/ThemeContext'

interface Props {
  values: number[]
  max?: number
  tone?: 'default' | 'warning' | 'danger'
  width?: number
  height?: number
}

function readCssVar(name: string, fallback: string) {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback
}

export default function Sparkline({
  values,
  max = 100,
  tone = 'default',
  width = 80,
  height = 24,
}: Props) {
  const { theme } = useTheme()
  const [color, setColor] = useState('#c8f542')

  useEffect(() => {
    const varName =
      tone === 'warning' ? '--c-sparkline-warning' : tone === 'danger' ? '--c-sparkline-danger' : '--c-sparkline-default'
    const fallback = tone === 'warning' ? '#F59E0B' : tone === 'danger' ? '#EF4444' : '#c8f542'
    setColor(readCssVar(varName, fallback))
  }, [theme, tone])

  if (values.length < 2) {
    return (
      <div style={{ width, height }} className="font-mono text-xs text-text-faint">
        --
      </div>
    )
  }

  const step = width / (values.length - 1)
  const points = values
    .map((v, i) => {
      const x = i * step
      const y = height - (Math.min(v, max) / max) * height
      return `${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')

  const last = values[values.length - 1]

  return (
    <svg width={width} height={height} className="overflow-visible">
      <polyline
        points={points}
        fill="none"
        stroke={color}
        strokeWidth="1.5"
        strokeLinejoin="round"
        strokeLinecap="round"
      />
      <circle
        cx={(values.length - 1) * step}
        cy={height - (Math.min(last, max) / max) * height}
        r="2"
        fill={color}
      />
    </svg>
  )
}
