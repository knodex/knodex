// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { HealthBadge } from './HealthBadge'

describe('HealthBadge', () => {
  it('renders Healthy status', () => {
    render(<HealthBadge health="Healthy" />)
    expect(screen.getByText('Healthy')).toBeInTheDocument()
  })

  it('renders Degraded status', () => {
    render(<HealthBadge health="Degraded" />)
    expect(screen.getByText('Degraded')).toBeInTheDocument()
  })

  it('renders Unhealthy status', () => {
    render(<HealthBadge health="Unhealthy" />)
    expect(screen.getByText('Unhealthy')).toBeInTheDocument()
  })

  it('renders Progressing status with animation', () => {
    const { container } = render(<HealthBadge health="Progressing" />)
    expect(screen.getByText('Progressing')).toBeInTheDocument()
    // Check for animate-spin class on the icon
    const svg = container.querySelector('svg')
    expect(svg).toHaveClass('animate-spin')
  })

  it('renders Unknown status', () => {
    render(<HealthBadge health="Unknown" />)
    expect(screen.getByText('Unknown')).toBeInTheDocument()
  })

  it('renders small size correctly', () => {
    const { container } = render(<HealthBadge health="Healthy" size="sm" />)
    const badge = container.firstChild as HTMLElement
    expect(badge).toHaveClass('px-2', 'py-0.5', 'text-xs')
  })

  it('renders medium size correctly', () => {
    const { container } = render(<HealthBadge health="Healthy" size="md" />)
    const badge = container.firstChild as HTMLElement
    expect(badge).toHaveClass('px-2.5', 'py-1', 'text-xs')
  })

  it('hides label when showLabel is false', () => {
    render(<HealthBadge health="Healthy" showLabel={false} />)
    expect(screen.queryByText('Healthy')).not.toBeInTheDocument()
  })

  it('shows label by default', () => {
    render(<HealthBadge health="Healthy" />)
    expect(screen.getByText('Healthy')).toBeInTheDocument()
  })

  it('applies correct color classes for each health status', () => {
    const { container, rerender } = render(<HealthBadge health="Healthy" />)
    expect(container.firstChild).toHaveClass('text-primary')

    rerender(<HealthBadge health="Degraded" />)
    expect(container.firstChild).toHaveClass('text-status-warning')

    rerender(<HealthBadge health="Unhealthy" />)
    expect(container.firstChild).toHaveClass('text-destructive')

    rerender(<HealthBadge health="Progressing" />)
    expect(container.firstChild).toHaveClass('text-status-info')

    rerender(<HealthBadge health="Unknown" />)
    expect(container.firstChild).toHaveClass('text-muted-foreground')
  })

  it('has rounded-full class for pill shape', () => {
    const { container } = render(<HealthBadge health="Healthy" />)
    expect(container.firstChild).toHaveClass('rounded-full')
  })

  it('renders with border', () => {
    const { container } = render(<HealthBadge health="Healthy" />)
    expect(container.firstChild).toHaveClass('border')
  })
})
