// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from './card'

describe('Card', () => {
  it('renders Card component', () => {
    render(<Card data-testid="card">Card content</Card>)
    const card = screen.getByTestId('card')
    expect(card).toBeInTheDocument()
    expect(card).toHaveClass('rounded-lg', 'border', 'shadow-sm')
  })

  it('applies custom className to Card', () => {
    render(<Card className="custom-card" data-testid="card">Content</Card>)
    expect(screen.getByTestId('card')).toHaveClass('custom-card')
  })

  it('forwards ref to Card', () => {
    const ref = vi.fn()
    render(<Card ref={ref}>Content</Card>)
    expect(ref).toHaveBeenCalled()
  })
})

describe('CardHeader', () => {
  it('renders CardHeader component', () => {
    render(<CardHeader data-testid="header">Header content</CardHeader>)
    const header = screen.getByTestId('header')
    expect(header).toBeInTheDocument()
    expect(header).toHaveClass('flex', 'flex-col', 'p-6')
  })

  it('applies custom className to CardHeader', () => {
    render(<CardHeader className="custom-header" data-testid="header">Header</CardHeader>)
    expect(screen.getByTestId('header')).toHaveClass('custom-header')
  })
})

describe('CardTitle', () => {
  it('renders CardTitle component', () => {
    render(<CardTitle>Title</CardTitle>)
    const title = screen.getByRole('heading', { level: 3 })
    expect(title).toBeInTheDocument()
    expect(title).toHaveTextContent('Title')
    expect(title).toHaveClass('text-2xl', 'font-semibold')
  })

  it('applies custom className to CardTitle', () => {
    render(<CardTitle className="custom-title">Title</CardTitle>)
    expect(screen.getByRole('heading')).toHaveClass('custom-title')
  })
})

describe('CardDescription', () => {
  it('renders CardDescription component', () => {
    render(<CardDescription>Description text</CardDescription>)
    const description = screen.getByText('Description text')
    expect(description).toBeInTheDocument()
    expect(description).toHaveClass('text-sm', 'text-muted-foreground')
  })
})

describe('CardContent', () => {
  it('renders CardContent component', () => {
    render(<CardContent data-testid="content">Content</CardContent>)
    const content = screen.getByTestId('content')
    expect(content).toBeInTheDocument()
    expect(content).toHaveClass('p-6', 'pt-0')
  })
})

describe('CardFooter', () => {
  it('renders CardFooter component', () => {
    render(<CardFooter data-testid="footer">Footer</CardFooter>)
    const footer = screen.getByTestId('footer')
    expect(footer).toBeInTheDocument()
    expect(footer).toHaveClass('flex', 'items-center', 'p-6', 'pt-0')
  })
})

describe('Card composition', () => {
  it('renders full card with all sub-components', () => {
    render(
      <Card data-testid="card">
        <CardHeader>
          <CardTitle>Card Title</CardTitle>
          <CardDescription>Card description</CardDescription>
        </CardHeader>
        <CardContent>
          <p>Card content</p>
        </CardContent>
        <CardFooter>
          <button>Action</button>
        </CardFooter>
      </Card>
    )

    expect(screen.getByTestId('card')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: /card title/i })).toBeInTheDocument()
    expect(screen.getByText('Card description')).toBeInTheDocument()
    expect(screen.getByText('Card content')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /action/i })).toBeInTheDocument()
  })
})
