import React from 'react';

const tagStyles: Record<string, { bg: string; text: string; label: string }> = {
  oss: {
    bg: 'var(--ifm-color-success-lightest, #e6f9ef)',
    text: 'var(--ifm-color-success-darkest, #1a7f37)',
    label: 'OSS',
  },
  enterprise: {
    bg: 'var(--ifm-color-primary-lightest, #e0e7ff)',
    text: 'var(--ifm-color-primary-darkest, #312e81)',
    label: 'Enterprise',
  },
};

const darkOverrides: Record<string, { bg: string; text: string }> = {
  oss: {
    bg: 'rgba(46, 160, 67, 0.15)',
    text: '#56d364',
  },
  enterprise: {
    bg: 'rgba(130, 140, 248, 0.15)',
    text: '#a5b4fc',
  },
};

interface ProductTagProps {
  tags: ('oss' | 'enterprise')[];
}

export default function ProductTag({ tags }: ProductTagProps): JSX.Element {
  return (
    <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem' }}>
      {tags.map((tag) => {
        const style = tagStyles[tag];
        if (!style) return null;
        return (
          <span
            key={tag}
            className="product-tag"
            data-tag={tag}
            style={{
              display: 'inline-block',
              padding: '0.15rem 0.5rem',
              borderRadius: '0.25rem',
              fontSize: '0.75rem',
              fontWeight: 600,
              letterSpacing: '0.025em',
              textTransform: 'uppercase',
              backgroundColor: style.bg,
              color: style.text,
            }}
          >
            {style.label}
          </span>
        );
      })}
    </div>
  );
}
