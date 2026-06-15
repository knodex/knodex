// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import { cn } from "@/lib/utils";

/**
 * AgentMarkdown renders UNTRUSTED agent prose as sanitized GFM markdown
 * (Story 50.5) — the reusable renderer for every agent chat surface (mirrors
 * AgentModelBadge, 50.4).
 *
 * Security boundary: agent output is LLM-generated over arbitrary cluster
 * strings. react-markdown builds a React element tree (never
 * dangerouslySetInnerHTML); raw HTML in the source is escaped to text because
 * we do NOT add rehype-raw; the default urlTransform strips dangerous link
 * protocols (javascript:/vbscript:/data:). Do NOT add rehype-raw and do NOT
 * override urlTransform to be more permissive — the XSS regression test
 * (AgentMarkdown.test.tsx) locks this in.
 *
 * Styled via the components map (design-system tokens) rather than a Tailwind
 * Typography dependency.
 */

// _node = the hast node react-markdown passes to overrides (passNode: true is
// hardcoded in react-markdown v10); drop it so it never reaches the DOM element
// as an unknown attribute.
const components: Components = {
  p: ({ node: _node, ...props }) => (
    <p className="leading-relaxed [&:not(:first-child)]:mt-2" {...props} />
  ),
  h1: ({ node: _node, ...props }) => (
    // eslint-disable-next-line jsx-a11y/heading-has-content
    <h1 className="text-base font-semibold mt-3 mb-1 first:mt-0" {...props} />
  ),
  h2: ({ node: _node, ...props }) => (
    // eslint-disable-next-line jsx-a11y/heading-has-content
    <h2 className="text-sm font-semibold mt-3 mb-1 first:mt-0" {...props} />
  ),
  h3: ({ node: _node, ...props }) => (
    // eslint-disable-next-line jsx-a11y/heading-has-content
    <h3 className="text-sm font-semibold mt-2 mb-1 first:mt-0" {...props} />
  ),
  ul: ({ node: _node, ...props }) => (
    <ul className="list-disc pl-5 space-y-0.5 my-2" {...props} />
  ),
  ol: ({ node: _node, ...props }) => (
    <ol className="list-decimal pl-5 space-y-0.5 my-2" {...props} />
  ),
  li: ({ node: _node, ...props }) => <li className="leading-relaxed" {...props} />,
  a: ({ node: _node, ...props }) => (
    // target=_blank + rel=noopener noreferrer: no reverse-tabnabbing (AC #5).
    // eslint-disable-next-line jsx-a11y/anchor-has-content
    <a
      className="text-primary underline underline-offset-2 hover:opacity-80"
      target="_blank"
      rel="noopener noreferrer"
      {...props}
    />
  ),
  strong: ({ node: _node, ...props }) => <strong className="font-semibold" {...props} />,
  em: ({ node: _node, ...props }) => <em className="italic" {...props} />,
  code: ({ node: _node, className, ...props }) => {
    // className carries `language-*` for fenced blocks; absent for inline code.
    const isBlock = !!className?.startsWith("language-");
    return (
      <code
        className={cn("font-mono text-xs", !isBlock && "bg-muted px-1 py-0.5 rounded", className)}
        {...props}
      />
    );
  },
  pre: ({ node: _node, ...props }) => (
    <pre
      className="bg-muted rounded-md p-3 my-2 overflow-x-auto text-xs font-mono"
      {...props}
    />
  ),
  table: ({ node: _node, ...props }) => (
    // The GFM pipe table is the headline fix — wrap for horizontal overflow.
    <div className="my-2 overflow-x-auto">
      <table className="w-full border-collapse text-xs" {...props} />
    </div>
  ),
  th: ({ node: _node, ...props }) => (
    <th
      className="border border-border/60 bg-muted/50 px-2 py-1 text-left font-semibold"
      {...props}
    />
  ),
  td: ({ node: _node, ...props }) => (
    <td className="border border-border/60 px-2 py-1 align-top" {...props} />
  ),
  blockquote: ({ node: _node, ...props }) => (
    <blockquote
      className="border-l-2 border-border/60 pl-3 my-2 text-muted-foreground"
      {...props}
    />
  ),
  hr: ({ node: _node, ...props }) => <hr className="my-3 border-border/60" {...props} />,
};

export function AgentMarkdown({
  children,
  className,
}: {
  children: string;
  className?: string;
}) {
  return (
    <div className={cn("text-sm break-words", className)}>
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {children}
      </ReactMarkdown>
    </div>
  );
}
