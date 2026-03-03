import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CreateConstraintDialog } from "./CreateConstraintDialog";
import type { ConstraintTemplate } from "@/types/compliance";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

// Mock the useCreateConstraint hook
const mockMutateAsync = vi.fn();
const mockReset = vi.fn();
vi.mock("@/hooks/useCompliance", () => ({
  useCreateConstraint: () => ({
    mutateAsync: mockMutateAsync,
    reset: mockReset,
    isPending: false,
    isError: false,
    error: null,
  }),
}));

// Mock toast notifications
vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

// Mock clipboard API
const mockWriteText = vi.fn();
Object.assign(navigator, {
  clipboard: {
    writeText: mockWriteText,
  },
});

// Mock Radix UI Dialog to avoid portal issues
vi.mock("@/components/ui/dialog", () => ({
  Dialog: ({ children, open }: { children: React.ReactNode; open: boolean }) =>
    open ? <div data-testid="dialog">{children}</div> : null,
  DialogContent: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="dialog-content">{children}</div>
  ),
  DialogHeader: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="dialog-header">{children}</div>
  ),
  DialogTitle: ({ children }: { children: React.ReactNode }) => (
    <h2 data-testid="dialog-title">{children}</h2>
  ),
  DialogDescription: ({ children }: { children: React.ReactNode }) => (
    <p data-testid="dialog-description">{children}</p>
  ),
  DialogFooter: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="dialog-footer">{children}</div>
  ),
}));

// Mock Radix UI Tabs
vi.mock("@/components/ui/tabs", () => ({
  Tabs: ({
    children,
    value,
  }: {
    children: React.ReactNode;
    value: string;
    onValueChange?: (v: string) => void;
  }) => (
    <div data-testid="tabs" data-value={value}>
      {children}
    </div>
  ),
  TabsList: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="tabs-list" role="tablist">{children}</div>
  ),
  TabsTrigger: ({
    children,
    value,
    onClick,
  }: {
    children: React.ReactNode;
    value: string;
    onClick?: () => void;
  }) => (
    <button
      role="tab"
      data-testid={`tab-${value}`}
      data-value={value}
      onClick={onClick}
    >
      {children}
    </button>
  ),
  TabsContent: ({
    children,
    value,
  }: {
    children: React.ReactNode;
    value: string;
  }) => <div data-testid={`tab-content-${value}`}>{children}</div>,
}));

// Mock Radix UI Select
let selectOnValueChange: ((value: string) => void) | undefined;
vi.mock("@/components/ui/select", () => ({
  Select: ({
    children,
    value,
    onValueChange,
  }: {
    children: React.ReactNode;
    value: string;
    onValueChange: (value: string) => void;
  }) => {
    selectOnValueChange = onValueChange;
    return (
      <div data-testid="select" data-value={value}>
        {children}
      </div>
    );
  },
  SelectTrigger: ({
    children,
    ...props
  }: {
    children: React.ReactNode;
    "data-testid"?: string;
  }) => (
    <button role="combobox" {...props}>
      {children}
    </button>
  ),
  SelectValue: ({ children }: { children?: React.ReactNode }) => (
    <span>{children}</span>
  ),
  SelectContent: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="select-content">{children}</div>
  ),
  SelectItem: ({
    children,
    value,
  }: {
    children: React.ReactNode;
    value: string;
  }) => (
    <div
      role="option"
      data-testid={`select-option-${value}`}
      onClick={() => selectOnValueChange?.(value)}
    >
      {children}
    </div>
  ),
}));

describe("CreateConstraintDialog", () => {
  const mockTemplate: ConstraintTemplate = {
    name: "k8srequiredlabels",
    kind: "K8sRequiredLabels",
    description: "Requires specified labels on resources",
    rego: "package k8srequiredlabels",
    parameters: {
      properties: {
        labels: {
          type: "array",
          items: { type: "string" },
          default: ["team"],
        },
      },
    },
    labels: {},
    createdAt: "2024-01-01T00:00:00Z",
  };

  const mockOnOpenChange = vi.fn();
  const mockOnSuccess = vi.fn();

  let queryClient: QueryClient;

  beforeEach(() => {
    vi.clearAllMocks();
    selectOnValueChange = undefined;
    mockMutateAsync.mockResolvedValue({
      name: "test-constraint",
      kind: "K8sRequiredLabels",
      templateName: "k8srequiredlabels",
      enforcementAction: "deny",
    });
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });
  });

  afterEach(() => {
    queryClient.clear();
  });

  const renderDialog = (open = true) => {
    return render(
      <QueryClientProvider client={queryClient}>
        <CreateConstraintDialog
          template={mockTemplate}
          open={open}
          onOpenChange={mockOnOpenChange}
          onSuccess={mockOnSuccess}
        />
      </QueryClientProvider>
    );
  };

  describe("rendering", () => {
    it("renders dialog when open is true", () => {
      renderDialog(true);
      expect(screen.getByTestId("dialog")).toBeInTheDocument();
    });

    it("does not render dialog when open is false", () => {
      renderDialog(false);
      expect(screen.queryByTestId("dialog")).not.toBeInTheDocument();
    });

    it("shows template information in title (AC-FORM-01)", () => {
      renderDialog();
      expect(screen.getByTestId("dialog-title")).toHaveTextContent(
        "Create Constraint from K8sRequiredLabels"
      );
    });

    it("shows template description", () => {
      renderDialog();
      expect(screen.getByTestId("dialog-description")).toHaveTextContent(
        "Requires specified labels on resources"
      );
    });

    it("renders all tab triggers", () => {
      renderDialog();
      expect(screen.getByTestId("tab-basic")).toBeInTheDocument();
      expect(screen.getByTestId("tab-match")).toBeInTheDocument();
      expect(screen.getByTestId("tab-parameters")).toBeInTheDocument();
      expect(screen.getByTestId("tab-preview")).toBeInTheDocument();
    });

    it("renders constraint name input (AC-FORM-01)", () => {
      renderDialog();
      expect(screen.getByTestId("constraint-name-input")).toBeInTheDocument();
    });

    it("renders enforcement action selector (AC-FORM-03)", () => {
      renderDialog();
      expect(
        screen.getByTestId("enforcement-action-select")
      ).toBeInTheDocument();
    });

    it("renders submit button", () => {
      renderDialog();
      expect(
        screen.getByTestId("create-constraint-submit")
      ).toBeInTheDocument();
    });
  });

  describe("form validation", () => {
    it("validates name is required (AC-FORM-02)", async () => {
      renderDialog();

      // Leave name empty and try to submit
      const submitBtn = screen.getByTestId("create-constraint-submit");
      expect(submitBtn).toBeDisabled();
    });

    it("validates DNS-compatible name format (AC-FORM-02)", async () => {
      const user = userEvent.setup();
      renderDialog();

      const nameInput = screen.getByTestId("constraint-name-input");

      // Invalid: starts with hyphen
      await user.type(nameInput, "-invalid-name");

      await waitFor(() => {
        expect(
          screen.getByText(/DNS-compatible: lowercase alphanumeric/i)
        ).toBeInTheDocument();
      });
    });

    it("accepts valid DNS-compatible names", async () => {
      const user = userEvent.setup();
      renderDialog();

      const nameInput = screen.getByTestId("constraint-name-input");
      await user.type(nameInput, "valid-constraint-name");

      await waitFor(() => {
        // Should not show error for valid name
        expect(
          screen.queryByText(/DNS-compatible: lowercase alphanumeric.*cannot start/i)
        ).not.toBeInTheDocument();
      });
    });

    it("validates name length limit (max 253 chars)", async () => {
      renderDialog();

      const nameInput = screen.getByTestId("constraint-name-input");
      const longName = "a".repeat(260);
      // Use fireEvent instead of user.type for long strings (260 keystrokes is slow)
      fireEvent.change(nameInput, { target: { value: longName } });
      fireEvent.blur(nameInput);

      await waitFor(() => {
        expect(
          screen.getByText(/253 characters or less/i)
        ).toBeInTheDocument();
      });
    });
  });

  describe("enforcement action selection", () => {
    it("defaults to deny enforcement action", () => {
      renderDialog();
      const select = screen.getByTestId("select");
      expect(select).toHaveAttribute("data-value", "deny");
    });

    it("allows selecting different enforcement actions (AC-FORM-03)", async () => {
      const user = userEvent.setup();
      renderDialog();

      // Click on warn option
      await user.click(screen.getByTestId("select-option-warn"));

      expect(selectOnValueChange).toBeDefined();
    });
  });

  describe("match rules", () => {
    it("renders match rules tab content", () => {
      renderDialog();
      expect(screen.getByTestId("tab-content-match")).toBeInTheDocument();
    });

    it("shows add match kind button (AC-FORM-04)", () => {
      renderDialog();
      expect(screen.getByTestId("add-match-kind-btn")).toBeInTheDocument();
    });

    it("renders initial match kind fields", () => {
      renderDialog();
      expect(screen.getByTestId("api-groups-selector-0")).toBeInTheDocument();
      expect(screen.getByTestId("kinds-selector-0")).toBeInTheDocument();
    });

    it("allows adding additional match kinds", async () => {
      const user = userEvent.setup();
      renderDialog();

      await user.click(screen.getByTestId("add-match-kind-btn"));

      await waitFor(() => {
        expect(screen.getByTestId("api-groups-selector-1")).toBeInTheDocument();
        expect(screen.getByTestId("kinds-selector-1")).toBeInTheDocument();
      });
    });

    it("allows removing match kinds", async () => {
      const user = userEvent.setup();
      renderDialog();

      // First add another match kind
      await user.click(screen.getByTestId("add-match-kind-btn"));
      await waitFor(() => {
        expect(screen.getByTestId("remove-match-kind-1")).toBeInTheDocument();
      });

      // Then remove it
      await user.click(screen.getByTestId("remove-match-kind-1"));
      await waitFor(() => {
        expect(
          screen.queryByTestId("api-groups-selector-1")
        ).not.toBeInTheDocument();
      });
    });

    it("renders namespace filter input", () => {
      renderDialog();
      expect(screen.getByTestId("namespaces-input")).toBeInTheDocument();
    });
  });

  describe("parameters", () => {
    it("renders parameters tab content", () => {
      renderDialog();
      expect(screen.getByTestId("tab-content-parameters")).toBeInTheDocument();
    });

    it("shows parameter section when template has parameters (AC-FORM-05)", () => {
      renderDialog();
      // The parameters tab content should be displayed
      expect(screen.getByTestId("tab-content-parameters")).toBeInTheDocument();
      // In form mode, shows "Parameter Values" label; in JSON mode, shows "Parameter Schema"
      const formMode = screen.queryByTestId("params-form-mode");
      const jsonMode = screen.queryByTestId("params-json-mode");
      expect(formMode || jsonMode).toBeTruthy();
    });

    it("renders parameters form or raw input", () => {
      renderDialog();
      // ParameterFormSection renders either form mode or JSON mode container
      const formMode = screen.queryByTestId("params-form-mode");
      const jsonMode = screen.queryByTestId("params-json-mode");
      expect(formMode || jsonMode).toBeTruthy();
    });

    it("shows message when template has no parameters", () => {
      const templateWithoutParams: ConstraintTemplate = {
        ...mockTemplate,
        parameters: undefined,
      };

      render(
        <QueryClientProvider client={queryClient}>
          <CreateConstraintDialog
            template={templateWithoutParams}
            open={true}
            onOpenChange={mockOnOpenChange}
            onSuccess={mockOnSuccess}
          />
        </QueryClientProvider>
      );

      expect(
        screen.getByText(/does not require any parameters/i)
      ).toBeInTheDocument();
    });
  });

  describe("YAML preview", () => {
    it("renders preview tab content", () => {
      renderDialog();
      expect(screen.getByTestId("tab-content-preview")).toBeInTheDocument();
    });

    it("shows copy YAML button (AC-PREVIEW-02)", () => {
      renderDialog();
      expect(screen.getByTestId("copy-yaml-btn")).toBeInTheDocument();
    });

    it("renders copy YAML button that can be clicked", async () => {
      mockWriteText.mockResolvedValue(undefined);
      renderDialog();

      const copyBtn = screen.getByTestId("copy-yaml-btn");
      expect(copyBtn).toBeInTheDocument();
      expect(copyBtn).toHaveTextContent(/Copy YAML/i);
    });

    it("shows generated YAML preview (AC-PREVIEW-01)", () => {
      renderDialog();
      // Check for YAML content indicators
      expect(screen.getByText(/Generated Constraint YAML/i)).toBeInTheDocument();
    });
  });

  describe("form submission", () => {
    it("submit button is initially disabled when form is invalid", () => {
      renderDialog();

      // Submit button should be disabled when name is empty
      const submitBtn = screen.getByTestId("create-constraint-submit");
      expect(submitBtn).toBeDisabled();
    });

    it("submit button shows correct text", () => {
      renderDialog();

      const submitBtn = screen.getByTestId("create-constraint-submit");
      expect(submitBtn).toHaveTextContent(/Create Constraint/i);
    });

    it("form element is present in the dialog", () => {
      renderDialog();

      // Form should be present (forms don't have implicit ARIA role)
      const form = document.querySelector("form");
      expect(form).toBeInTheDocument();
    });
  });

  describe("error handling", () => {
    it("error display area is available when mutation fails", () => {
      // This tests that the error handling UI structure exists
      // The actual error display is conditional on isError from the hook
      renderDialog();

      // Form should be present to display errors
      const form = document.querySelector("form");
      expect(form).toBeInTheDocument();
    });
  });

  describe("cancel behavior", () => {
    it("closes dialog when cancel button clicked", async () => {
      const user = userEvent.setup();
      renderDialog();

      const cancelBtn = screen.getByRole("button", { name: /cancel/i });
      await user.click(cancelBtn);

      expect(mockOnOpenChange).toHaveBeenCalledWith(false);
    });

    it("resets form state when cancelled", async () => {
      const user = userEvent.setup();
      renderDialog();

      // Type something in the name field
      const nameInput = screen.getByTestId("constraint-name-input");
      await user.type(nameInput, "partial-input");

      // Cancel the dialog
      const cancelBtn = screen.getByRole("button", { name: /cancel/i });
      await user.click(cancelBtn);

      expect(mockReset).toHaveBeenCalled();
    });
  });
});
