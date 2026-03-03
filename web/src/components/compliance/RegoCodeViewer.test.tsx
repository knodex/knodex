import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RegoCodeViewer } from "./RegoCodeViewer";

// Mock clipboard API
const mockWriteText = vi.fn();

describe("RegoCodeViewer", () => {
  const sampleRego = `package k8srequiredlabels

# Comment about the violation rule
violation[{"msg": msg}] {
  provided := {label | input.review.object.metadata.labels[label]}
  required := {label | label := input.parameters.labels[_]}
  missing := required - provided
  count(missing) > 0
  msg := sprintf("Missing required labels: %v", [missing])
}`;

  beforeEach(() => {
    mockWriteText.mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      value: {
        writeText: mockWriteText,
      },
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("renders code content (AC-TPL-06)", () => {
    const { container } = render(<RegoCodeViewer code={sampleRego} />);

    // Check that the code content is in the document
    expect(container.textContent).toContain("package");
    expect(container.textContent).toContain("k8srequiredlabels");
    expect(container.textContent).toContain("violation");
  });

  it("renders title when provided", () => {
    render(<RegoCodeViewer code={sampleRego} title="Rego Policy" />);

    expect(screen.getByText("Rego Policy")).toBeInTheDocument();
  });

  it("renders line numbers by default (AC-TPL-06)", () => {
    render(<RegoCodeViewer code={sampleRego} />);

    // Should show line numbers
    expect(screen.getByText("1")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
  });

  it("hides line numbers when showLineNumbers is false", () => {
    const { container } = render(<RegoCodeViewer code={sampleRego} showLineNumbers={false} />);

    // Line numbers should not be present as separate elements
    // The content should still be there
    expect(container.textContent).toContain("package");
    expect(container.textContent).toContain("k8srequiredlabels");

    // When showLineNumbers is false, no table is rendered
    expect(container.querySelector("table")).not.toBeInTheDocument();
  });

  it("renders copy button", () => {
    render(<RegoCodeViewer code={sampleRego} />);

    // Should have a copy button with "Copy" text
    const copyButton = screen.getByRole("button", { name: /copy/i });
    expect(copyButton).toBeInTheDocument();
  });

  it("copies code to clipboard when copy button is clicked", async () => {
    const user = userEvent.setup();
    const localMockWriteText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: localMockWriteText },
      writable: true,
      configurable: true,
    });

    render(<RegoCodeViewer code={sampleRego} />);

    const copyButton = screen.getByRole("button", { name: /copy/i });
    await user.click(copyButton);

    expect(localMockWriteText).toHaveBeenCalledWith(sampleRego);
  });

  it("highlights Rego keywords (AC-TPL-06)", () => {
    const { container } = render(<RegoCodeViewer code={sampleRego} />);

    // Keywords like 'package' should have highlighting classes
    const code = container.querySelector("code");
    expect(code).toBeInTheDocument();
    // Check that keyword highlighting class is applied (text-purple-500 for keywords)
    const keywordSpans = container.querySelectorAll('[class*="text-purple"]');
    expect(keywordSpans.length).toBeGreaterThan(0);
  });

  it("highlights comments (AC-TPL-06)", () => {
    const { container } = render(<RegoCodeViewer code={sampleRego} />);

    // Comments starting with # should be highlighted with italic class and gray text
    expect(container.textContent).toContain("Comment about the violation rule");
    // Comments have both text-gray-500 and italic classes
    const commentSpans = container.querySelectorAll('[class*="italic"]');
    expect(commentSpans.length).toBeGreaterThan(0);
  });

  it("highlights strings (AC-TPL-06)", () => {
    const { container } = render(<RegoCodeViewer code={sampleRego} />);

    // String literals should be present with green styling
    expect(container.textContent).toContain("Missing required labels");
    const stringSpans = container.querySelectorAll('[class*="text-green"]');
    expect(stringSpans.length).toBeGreaterThan(0);
  });

  it("applies maxHeight prop", () => {
    const { container } = render(
      <RegoCodeViewer code={sampleRego} maxHeight="200px" />
    );

    // The scroll container should have max-height applied
    const scrollDiv = container.querySelector(".overflow-auto");
    expect(scrollDiv).toHaveStyle({ maxHeight: "200px" });
  });

  it("applies custom className", () => {
    const { container } = render(
      <RegoCodeViewer code={sampleRego} className="custom-class" />
    );

    expect(container.firstChild).toHaveClass("custom-class");
  });

  it("handles empty code", () => {
    const { container } = render(<RegoCodeViewer code="" />);

    // Should render without crashing
    const codeElement = container.querySelector("code");
    expect(codeElement).toBeInTheDocument();
  });

  it("handles code with special characters", () => {
    const codeWithSpecialChars = `package test
# Check for <tag> and & symbol
violation[{"msg": msg}] {
  msg := "Value is < 10 && > 0"
}`;

    const { container } = render(<RegoCodeViewer code={codeWithSpecialChars} />);

    expect(container.textContent).toContain("Check for");
    expect(container.textContent).toContain("<tag>");
  });

  it("handles very long lines", () => {
    const longLineCode = `package test
violation[{"msg": msg}] { msg := "This is a very very very very very very very very very very very very very very very very very very long line" }`;

    const { container } = render(<RegoCodeViewer code={longLineCode} />);

    // Should have overflow handling (scrollable) - the parent div has overflow-auto
    const scrollContainer = container.querySelector(".overflow-auto");
    expect(scrollContainer).toBeInTheDocument();
  });

  it("handles multiline strings", () => {
    const multilineCode = `package test
msg := \`
  This is a
  multiline string
\``;

    const { container } = render(<RegoCodeViewer code={multilineCode} />);

    expect(container.textContent).toContain("multiline string");
  });

  it("preserves indentation", () => {
    const indentedCode = `package test

violation[{"msg": msg}] {
    some x
    input.items[x]
    not input.allowed[x]
}`;

    const { container } = render(<RegoCodeViewer code={indentedCode} />);

    // The code should contain the indented content
    expect(container.textContent).toContain("some x");
    // The table cells with code have whitespace-pre class
    const whitespaceElements = container.querySelectorAll('[class*="whitespace-pre"]');
    expect(whitespaceElements.length).toBeGreaterThan(0);
  });

  it("shows default title when not provided", () => {
    render(<RegoCodeViewer code={sampleRego} />);

    expect(screen.getByText("Rego Policy")).toBeInTheDocument();
  });

  it("shows 'Copied' after clicking copy button", async () => {
    const user = userEvent.setup();
    render(<RegoCodeViewer code={sampleRego} />);

    const copyButton = screen.getByRole("button", { name: /copy/i });
    await user.click(copyButton);

    // After clicking, should show "Copied"
    expect(screen.getByText("Copied")).toBeInTheDocument();
  });
});
