// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Separator } from './separator'

describe('Separator', () => {
  it('renders with horizontal orientation by default', () => {
    render(<Separator data-testid="separator" />)
    const separator = screen.getByTestId('separator')
    expect(separator).toBeInTheDocument()
    expect(separator).toHaveClass('h-[1px]')
    expect(separator).toHaveClass('w-full')
  })

  it('renders with vertical orientation', () => {
    render(<Separator orientation="vertical" data-testid="separator" />)
    const separator = screen.getByTestId('separator')
    expect(separator).toHaveClass('h-full')
    expect(separator).toHaveClass('w-[1px]')
  })

  it('applies bg-border class', () => {
    render(<Separator data-testid="separator" />)
    const separator = screen.getByTestId('separator')
    expect(separator).toHaveClass('bg-border')
  })

  it('has role none when decorative is true (default)', () => {
    render(<Separator data-testid="separator" />)
    const separator = screen.getByTestId('separator')
    expect(separator).toHaveAttribute('role', 'none')
  })

  it('has role separator when decorative is false', () => {
    render(<Separator decorative={false} data-testid="separator" />)
    const separator = screen.getByTestId('separator')
    expect(separator).toHaveAttribute('role', 'separator')
    expect(separator).toHaveAttribute('aria-orientation', 'horizontal')
  })

  it('applies custom className', () => {
    render(<Separator className="my-4" data-testid="separator" />)
    const separator = screen.getByTestId('separator')
    expect(separator).toHaveClass('my-4')
  })

  it('has shrink-0 class', () => {
    render(<Separator data-testid="separator" />)
    const separator = screen.getByTestId('separator')
    expect(separator).toHaveClass('shrink-0')
  })
})
