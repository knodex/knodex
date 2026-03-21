// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { KeyValueEditor } from "./KeyValueEditor";
import type { KeyValuePair } from "./keyValueTypes";

describe("KeyValueEditor", () => {
  it("toggles value visibility", () => {
    const pairs: KeyValuePair[] = [
      { id: 1, key: "SECRET_KEY", value: "secret-value", visible: false },
    ];
    const onChange = vi.fn();

    render(<KeyValueEditor pairs={pairs} onChange={onChange} />);

    // Value should be masked (password input)
    const valueInput = screen.getByLabelText("Value 1");
    expect(valueInput).toHaveAttribute("type", "password");

    // Click toggle
    fireEvent.click(screen.getByLabelText("Show value"));

    // onChange should be called with visible=true
    expect(onChange).toHaveBeenCalledWith([
      { id: 1, key: "SECRET_KEY", value: "secret-value", visible: true },
    ]);
  });

  it("adds new key-value pair", () => {
    const pairs: KeyValuePair[] = [
      { id: 1, key: "KEY1", value: "val1", visible: false },
    ];
    const onChange = vi.fn();

    render(<KeyValueEditor pairs={pairs} onChange={onChange} />);

    fireEvent.click(screen.getByText("Add Key-Value Pair"));

    const call = onChange.mock.calls[0][0];
    expect(call).toHaveLength(2);
    expect(call[0]).toEqual({ id: 1, key: "KEY1", value: "val1", visible: false });
    expect(call[1]).toMatchObject({ key: "", value: "", visible: false });
    expect(call[1].id).toEqual(expect.any(Number));
  });

  it("removes a key-value pair", () => {
    const pairs: KeyValuePair[] = [
      { id: 1, key: "KEY1", value: "val1", visible: false },
      { id: 2, key: "KEY2", value: "val2", visible: false },
    ];
    const onChange = vi.fn();

    render(<KeyValueEditor pairs={pairs} onChange={onChange} />);

    const removeButtons = screen.getAllByLabelText("Remove row");
    fireEvent.click(removeButtons[0]);

    expect(onChange).toHaveBeenCalledWith([
      { id: 2, key: "KEY2", value: "val2", visible: false },
    ]);
  });

  it("disables remove when only one row", () => {
    const pairs: KeyValuePair[] = [
      { id: 1, key: "KEY1", value: "val1", visible: false },
    ];
    const onChange = vi.fn();

    render(<KeyValueEditor pairs={pairs} onChange={onChange} />);

    const removeButton = screen.getByLabelText("Remove row");
    expect(removeButton).toBeDisabled();
  });
});
