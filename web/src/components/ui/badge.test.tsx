// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Badge } from './badge'

describe('Badge', () => {
  it('renders with default variant', () => {
    render(<Badge>Default</Badge>)
    const badge = screen.getByText('Default')
    expect(badge).toBeInTheDocument()
    expect(badge).toHaveClass('bg-primary')
  })

  it('renders with secondary variant', () => {
    render(<Badge variant="secondary">Secondary</Badge>)
    const badge = screen.getByText('Secondary')
    expect(badge).toHaveClass('bg-secondary')
  })

  it('renders with destructive variant', () => {
    render(<Badge variant="destructive">Error</Badge>)
    const badge = screen.getByText('Error')
    expect(badge).toHaveClass('bg-destructive')
  })

  it('renders with outline variant', () => {
    render(<Badge variant="outline">Outline</Badge>)
    const badge = screen.getByText('Outline')
    expect(badge).toHaveClass('text-foreground')
  })

  it('applies custom className', () => {
    render(<Badge className="custom-badge">Custom</Badge>)
    const badge = screen.getByText('Custom')
    expect(badge).toHaveClass('custom-badge')
  })

  it('renders children correctly', () => {
    render(
      <Badge>
        <span>Child element</span>
      </Badge>
    )
    expect(screen.getByText('Child element')).toBeInTheDocument()
  })

  it('has rounded-full class for pill shape', () => {
    render(<Badge>Pill</Badge>)
    const badge = screen.getByText('Pill')
    expect(badge).toHaveClass('rounded-full')
  })
})
