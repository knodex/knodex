// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Input } from './input'

describe('Input', () => {
  it('renders input element', () => {
    render(<Input placeholder="Enter text" />)
    const input = screen.getByPlaceholderText('Enter text')
    expect(input).toBeInTheDocument()
  })

  it('renders with different types', () => {
    const { rerender } = render(<Input type="text" data-testid="input" />)
    expect(screen.getByTestId('input')).toHaveAttribute('type', 'text')

    rerender(<Input type="password" data-testid="input" />)
    expect(screen.getByTestId('input')).toHaveAttribute('type', 'password')

    rerender(<Input type="email" data-testid="input" />)
    expect(screen.getByTestId('input')).toHaveAttribute('type', 'email')

    rerender(<Input type="number" data-testid="input" />)
    expect(screen.getByTestId('input')).toHaveAttribute('type', 'number')
  })

  it('handles value changes', async () => {
    const user = userEvent.setup()
    render(<Input data-testid="input" />)

    const input = screen.getByTestId('input')
    await user.type(input, 'Hello World')

    expect(input).toHaveValue('Hello World')
  })

  it('handles onChange callback', () => {
    const handleChange = vi.fn()
    render(<Input onChange={handleChange} data-testid="input" />)

    fireEvent.change(screen.getByTestId('input'), {
      target: { value: 'test' },
    })

    expect(handleChange).toHaveBeenCalled()
  })

  it('can be disabled', () => {
    render(<Input disabled data-testid="input" />)
    const input = screen.getByTestId('input')
    expect(input).toBeDisabled()
    expect(input).toHaveClass('disabled:cursor-not-allowed', 'disabled:opacity-50')
  })

  it('applies custom className', () => {
    render(<Input className="custom-input" data-testid="input" />)
    expect(screen.getByTestId('input')).toHaveClass('custom-input')
  })

  it('forwards ref correctly', () => {
    const ref = vi.fn()
    render(<Input ref={ref} />)
    expect(ref).toHaveBeenCalled()
  })

  it('has correct base styles', () => {
    render(<Input data-testid="input" />)
    const input = screen.getByTestId('input')
    expect(input).toHaveClass(
      'flex',
      'h-10',
      'w-full',
      'rounded-md',
      'border'
    )
  })

  it('supports placeholder text', () => {
    render(<Input placeholder="Type here..." />)
    expect(screen.getByPlaceholderText('Type here...')).toBeInTheDocument()
  })

  it('supports controlled input', () => {
    const { rerender } = render(<Input value="initial" readOnly data-testid="input" />)
    expect(screen.getByTestId('input')).toHaveValue('initial')

    rerender(<Input value="updated" readOnly data-testid="input" />)
    expect(screen.getByTestId('input')).toHaveValue('updated')
  })
})
