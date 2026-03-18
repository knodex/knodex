// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { createElement } from "react";
import { TabBar } from "./TabBar";
import type { Tab } from "@/hooks/useDynamicTabs";

const icon = createElement("span", { "data-testid": "icon" }, "icon");

const tabs: Tab<"first" | "second" | "third">[] = [
  { id: "first", label: "First", icon },
  { id: "second", label: "Second", icon },
  { id: "third", label: "Third", icon },
];

describe("TabBar", () => {
  it("renders all tabs", () => {
    render(<TabBar tabs={tabs} activeTab="first" onChange={() => {}} />);
    expect(screen.getByText("First")).toBeInTheDocument();
    expect(screen.getByText("Second")).toBeInTheDocument();
    expect(screen.getByText("Third")).toBeInTheDocument();
  });

  it("marks active tab with aria-selected=true", () => {
    render(<TabBar tabs={tabs} activeTab="second" onChange={() => {}} />);
    const secondTab = screen.getByRole("tab", { name: /Second/ });
    expect(secondTab).toHaveAttribute("aria-selected", "true");

    const firstTab = screen.getByRole("tab", { name: /First/ });
    expect(firstTab).toHaveAttribute("aria-selected", "false");
  });

  it("applies active CSS class to active tab", () => {
    render(<TabBar tabs={tabs} activeTab="first" onChange={() => {}} />);
    const activeButton = screen.getByRole("tab", { name: /First/ });
    expect(activeButton.className).toContain("border-primary");

    const inactiveButton = screen.getByRole("tab", { name: /Second/ });
    expect(inactiveButton.className).toContain("border-transparent");
  });

  it("calls onChange when a tab is clicked", () => {
    const onChange = vi.fn();
    render(<TabBar tabs={tabs} activeTab="first" onChange={onChange} />);
    fireEvent.click(screen.getByRole("tab", { name: /Third/ }));
    expect(onChange).toHaveBeenCalledWith("third");
  });

  it("renders tab icons", () => {
    render(<TabBar tabs={tabs} activeTab="first" onChange={() => {}} />);
    const icons = screen.getAllByTestId("icon");
    expect(icons).toHaveLength(3);
  });

  it("sets correct id on tab buttons", () => {
    render(<TabBar tabs={tabs} activeTab="first" onChange={() => {}} />);
    expect(screen.getByRole("tab", { name: /First/ })).toHaveAttribute("id", "tab-first");
    expect(screen.getByRole("tab", { name: /Second/ })).toHaveAttribute("id", "tab-second");
  });

  it("has role=tablist on nav element", () => {
    render(<TabBar tabs={tabs} activeTab="first" onChange={() => {}} />);
    expect(screen.getByRole("tablist")).toBeInTheDocument();
  });

  it("tab buttons have type=button", () => {
    render(<TabBar tabs={tabs} activeTab="first" onChange={() => {}} />);
    screen.getAllByRole("tab").forEach((btn) => {
      expect(btn).toHaveAttribute("type", "button");
    });
  });

  it("does not set aria-controls (only active panel exists in DOM)", () => {
    render(<TabBar tabs={tabs} activeTab="first" onChange={() => {}} />);
    screen.getAllByRole("tab").forEach((btn) => {
      expect(btn).not.toHaveAttribute("aria-controls");
    });
  });

  it("active tab has tabIndex=0, inactive tabs have tabIndex=-1", () => {
    render(<TabBar tabs={tabs} activeTab="second" onChange={() => {}} />);
    expect(screen.getByRole("tab", { name: /Second/ })).toHaveAttribute("tabindex", "0");
    expect(screen.getByRole("tab", { name: /First/ })).toHaveAttribute("tabindex", "-1");
    expect(screen.getByRole("tab", { name: /Third/ })).toHaveAttribute("tabindex", "-1");
  });

  it("ArrowRight moves to next tab", () => {
    const onChange = vi.fn();
    render(<TabBar tabs={tabs} activeTab="first" onChange={onChange} />);
    const firstTab = screen.getByRole("tab", { name: /First/ });
    act(() => { firstTab.focus(); });
    fireEvent.keyDown(firstTab, { key: "ArrowRight" });
    expect(onChange).toHaveBeenCalledWith("second");
  });

  it("ArrowLeft wraps to last tab from first", () => {
    const onChange = vi.fn();
    render(<TabBar tabs={tabs} activeTab="first" onChange={onChange} />);
    const firstTab = screen.getByRole("tab", { name: /First/ });
    act(() => { firstTab.focus(); });
    fireEvent.keyDown(firstTab, { key: "ArrowLeft" });
    expect(onChange).toHaveBeenCalledWith("third");
  });

  it("Home moves to first tab", () => {
    const onChange = vi.fn();
    render(<TabBar tabs={tabs} activeTab="third" onChange={onChange} />);
    const thirdTab = screen.getByRole("tab", { name: /Third/ });
    act(() => { thirdTab.focus(); });
    fireEvent.keyDown(thirdTab, { key: "Home" });
    expect(onChange).toHaveBeenCalledWith("first");
  });

  it("End moves to last tab", () => {
    const onChange = vi.fn();
    render(<TabBar tabs={tabs} activeTab="first" onChange={onChange} />);
    const firstTab = screen.getByRole("tab", { name: /First/ });
    act(() => { firstTab.focus(); });
    fireEvent.keyDown(firstTab, { key: "End" });
    expect(onChange).toHaveBeenCalledWith("third");
  });
});
