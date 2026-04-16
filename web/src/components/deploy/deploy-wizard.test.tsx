// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import {
  createMemoryRouter,
  RouterProvider,
} from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import DeployWizardRoute from "@/routes/DeployWizard";
import { ProjectStep } from "./project-step";
import { ConfigureStep } from "./configure-step";
import { DiscardDialog } from "./discard-dialog";
import type { CatalogRGD, FormSchema, SchemaResponse } from "@/types/rgd";
import type { Project, ProjectListResponse } from "@/types/project";

// --- Mocks ---

const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual("react-router-dom");
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

const mockRGD: CatalogRGD = {
  name: "test-rgd",
  title: "Test RGD",
  namespace: "default",
  description: "A test RGD",
  tags: [],
  category: "database",
  instances: 0,
  status: "Active",
  kind: "TestRGD",
  labels: {},
  createdAt: new Date().toISOString(),
  updatedAt: new Date().toISOString(),
};

const mockSchema: FormSchema = {
  name: "test-rgd",
  namespace: "default",
  group: "test.knodex.io",
  kind: "TestRGD",
  version: "v1alpha1",
  properties: {
    replicas: {
      type: "integer",
      title: "Replicas",
      description: "Number of replicas",
      default: 3,
      minimum: 1,
      maximum: 10,
    },
    dbName: {
      type: "string",
      title: "Database Name",
      description: "Name of the database to create",
    },
  },
  required: ["dbName"],
};

const mockSchemaResponse: SchemaResponse = {
  rgd: "test-rgd",
  schema: mockSchema,
  crdFound: true,
};

const mockProjects: Project[] = [
  {
    name: "alpha",
    description: "Alpha project",
    destinations: [{ namespace: "alpha-ns" }],
    roles: [],
    resourceVersion: "1",
    createdAt: new Date().toISOString(),
  },
  {
    name: "beta",
    description: "Beta project",
    destinations: [{ namespace: "beta-ns" }],
    roles: [],
    resourceVersion: "1",
    createdAt: new Date().toISOString(),
  },
];

const mockSingleProject: Project[] = [
  {
    name: "only-project",
    description: "The only project",
    destinations: [{ namespace: "only-ns" }],
    roles: [],
    resourceVersion: "1",
    createdAt: new Date().toISOString(),
  },
];

// Mock hooks
vi.mock("@/hooks/useRGDs", () => ({
  useRGD: vi.fn(),
  useRGDSchema: vi.fn(),
}));

vi.mock("@/hooks/useProjects", () => ({
  useProjects: vi.fn(),
}));

vi.mock("@/hooks/useNamespaces", () => ({
  useProjectNamespaces: vi.fn(),
}));

vi.mock("@/hooks/useAuth", () => ({
  useCurrentProject: vi.fn(() => null),
  matchesNamespacePattern: vi.fn((pattern: string | undefined, ns: string) => {
    if (!pattern) return false;
    if (pattern === "*") return true;
    if (pattern === ns) return true;
    if (pattern.endsWith("*")) return ns.startsWith(pattern.slice(0, -1));
    return false;
  }),
}));

vi.mock("@/stores/userStore", () => ({
  useUserStore: vi.fn((selector: (state: { roles: Record<string, string> }) => unknown) =>
    selector({ roles: {} })
  ),
}));

vi.mock("@/api/compliance", () => ({
  validateCompliance: vi.fn(),
}));

vi.mock("@/api/rgd", () => ({
  createInstance: vi.fn(),
}));

import { useRGD, useRGDSchema } from "@/hooks/useRGDs";
import { useProjects } from "@/hooks/useProjects";
import { useProjectNamespaces } from "@/hooks/useNamespaces";

const mockUseRGD = vi.mocked(useRGD);
const mockUseRGDSchema = vi.mocked(useRGDSchema);
const mockUseProjects = vi.mocked(useProjects);
const mockUseProjectNamespaces = vi.mocked(useProjectNamespaces);

function createQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
}

/**
 * Render using a data router (required for useBlocker)
 */
function renderWithDataRouter(
  element: React.ReactElement,
  { initialRoute = "/deploy/test-rgd" } = {}
) {
  const queryClient = createQueryClient();
  const router = createMemoryRouter(
    [
      { path: "/deploy/:rgdName", element },
      { path: "/catalog", element: <div>Catalog Page</div> },
      { path: "/catalog/:rgdName", element: <div>RGD Detail</div> },
    ],
    { initialEntries: [initialRoute] }
  );
  return render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
}

function setupDefaultMocks(projectList = mockProjects) {
  mockUseRGD.mockReturnValue({
    data: mockRGD,
    isLoading: false,
    error: null,
  } as ReturnType<typeof useRGD>);

  mockUseRGDSchema.mockReturnValue({
    data: mockSchemaResponse,
    isLoading: false,
    error: null,
  } as ReturnType<typeof useRGDSchema>);

  mockUseProjects.mockReturnValue({
    data: { items: projectList, totalCount: projectList.length } as ProjectListResponse,
    isLoading: false,
    error: null,
  } as ReturnType<typeof useProjects>);

  mockUseProjectNamespaces.mockReturnValue({
    data: ["default", "staging"],
    isLoading: false,
    error: null,
  } as ReturnType<typeof useProjectNamespaces>);
}

beforeEach(() => {
  vi.clearAllMocks();
  mockNavigate.mockClear();
});

// ==========================================
// ProjectStep Component Tests
// ==========================================

describe("ProjectStep", () => {
  it("renders project and namespace dropdowns", () => {
    render(
      <ProjectStep
        projects={mockProjects}
        selectedProject=""
        onProjectChange={vi.fn()}
        namespaces={["default", "staging"]}
        selectedNamespace=""
        onNamespaceChange={vi.fn()}
      />
    );

    expect(screen.getByTestId("project-step")).toBeInTheDocument();
    expect(screen.getByTestId("project-select")).toBeInTheDocument();
    expect(screen.getByTestId("namespace-select")).toBeInTheDocument();
  });

  it("hides namespace dropdown for cluster-scoped RGDs", () => {
    render(
      <ProjectStep
        projects={mockProjects}
        selectedProject="alpha"
        onProjectChange={vi.fn()}
        namespaces={[]}
        selectedNamespace=""
        onNamespaceChange={vi.fn()}
        isClusterScoped={true}
      />
    );

    expect(screen.getByTestId("project-select")).toBeInTheDocument();
    expect(screen.queryByTestId("namespace-select")).not.toBeInTheDocument();
  });

  it("disables namespace when no project selected", () => {
    render(
      <ProjectStep
        projects={mockProjects}
        selectedProject=""
        onProjectChange={vi.fn()}
        namespaces={[]}
        selectedNamespace=""
        onNamespaceChange={vi.fn()}
      />
    );

    const namespaceTrigger = screen.getByTestId("namespace-select");
    expect(namespaceTrigger).toBeDisabled();
  });
});

// ==========================================
// ConfigureStep Component Tests
// ==========================================

describe("ConfigureStep", () => {
  it("renders form fields from schema", () => {
    render(<ConfigureStep schema={mockSchema} onValuesChange={vi.fn()} />);

    expect(screen.getByTestId("configure-step")).toBeInTheDocument();
    expect(screen.getByTestId("field-replicas")).toBeInTheDocument();
    expect(screen.getByTestId("field-dbName")).toBeInTheDocument();
  });

  it("renders field labels from schema", () => {
    render(<ConfigureStep schema={mockSchema} onValuesChange={vi.fn()} />);

    expect(screen.getByText("Replicas")).toBeInTheDocument();
    expect(screen.getByText("Database Name")).toBeInTheDocument();
  });

  it("pre-fills default values from schema", () => {
    render(<ConfigureStep schema={mockSchema} onValuesChange={vi.fn()} />);

    const replicasInput = screen.getByTestId("input-replicas") as HTMLInputElement;
    expect(replicasInput.value).toBe("3");
  });

  it("calls onValuesChange on mount with defaults", async () => {
    const onValuesChange = vi.fn();
    render(<ConfigureStep schema={mockSchema} onValuesChange={onValuesChange} />);

    await waitFor(() => {
      expect(onValuesChange).toHaveBeenCalled();
    });
  });
});

// ==========================================
// DiscardDialog Component Tests
// ==========================================

describe("DiscardDialog", () => {
  function renderDiscardDialog(hasUnsavedChanges: boolean) {
    const router = createMemoryRouter(
      [{ path: "/", element: <DiscardDialog hasUnsavedChanges={hasUnsavedChanges} /> }],
      { initialEntries: ["/"] }
    );
    return render(<RouterProvider router={router} />);
  }

  it("does not render when no unsaved changes", () => {
    renderDiscardDialog(false);
    expect(screen.queryByText("Discard changes?")).not.toBeInTheDocument();
  });

  it("sets beforeunload handler when changes exist", () => {
    const addSpy = vi.spyOn(window, "addEventListener");
    renderDiscardDialog(true);
    expect(addSpy).toHaveBeenCalledWith("beforeunload", expect.any(Function));
    addSpy.mockRestore();
  });

  it("removes beforeunload handler on cleanup", () => {
    const removeSpy = vi.spyOn(window, "removeEventListener");
    const { unmount } = renderDiscardDialog(true);
    unmount();
    expect(removeSpy).toHaveBeenCalledWith("beforeunload", expect.any(Function));
    removeSpy.mockRestore();
  });
});

// ==========================================
// DeployWizardRoute Integration Tests
// ==========================================

describe("DeployWizardRoute", () => {
  it("shows loading skeleton while data is loading", () => {
    mockUseRGD.mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    } as ReturnType<typeof useRGD>);
    mockUseRGDSchema.mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    } as ReturnType<typeof useRGDSchema>);
    mockUseProjects.mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    } as ReturnType<typeof useProjects>);
    mockUseProjectNamespaces.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useProjectNamespaces>);

    renderWithDataRouter(<DeployWizardRoute />);
    expect(screen.getByTestId("page-skeleton")).toBeInTheDocument();
  });

  it("shows error when RGD not found", () => {
    mockUseRGD.mockReturnValue({
      data: null,
      isLoading: false,
      error: new Error("not found"),
    } as unknown as ReturnType<typeof useRGD>);
    mockUseRGDSchema.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useRGDSchema>);
    mockUseProjects.mockReturnValue({
      data: { items: [], totalCount: 0 } as ProjectListResponse,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useProjects>);
    mockUseProjectNamespaces.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useProjectNamespaces>);

    renderWithDataRouter(<DeployWizardRoute />);
    expect(screen.getByText("RGD Not Found")).toBeInTheDocument();
  });

  it("renders wizard with project and configure steps for multiple projects", () => {
    setupDefaultMocks(mockProjects);
    renderWithDataRouter(<DeployWizardRoute />);

    expect(screen.getByTestId("step-wizard")).toBeInTheDocument();
    // Step labels are rendered as text within the wizard (Project label also on form)
    expect(screen.getAllByText("Project").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("Configure")).toBeInTheDocument();
    // Project step content should be visible (first step)
    expect(screen.getByTestId("project-step")).toBeInTheDocument();
  });

  it("auto-skips project step when single project", () => {
    setupDefaultMocks(mockSingleProject);
    renderWithDataRouter(<DeployWizardRoute />);

    expect(screen.getByTestId("step-wizard")).toBeInTheDocument();
    // Only Configure step, no Project step
    expect(screen.queryByText("Project")).not.toBeInTheDocument();
    expect(screen.getByTestId("configure-step")).toBeInTheDocument();
  });

  it("renders breadcrumb with Catalog > RGD name > Deploy", () => {
    setupDefaultMocks();
    renderWithDataRouter(<DeployWizardRoute />);

    expect(screen.getByText("Catalog")).toBeInTheDocument();
    expect(screen.getByText("Test RGD")).toBeInTheDocument();
    expect(screen.getByText("Deploy")).toBeInTheDocument();
  });

  it("renders page title with RGD name", () => {
    setupDefaultMocks();
    renderWithDataRouter(<DeployWizardRoute />);

    expect(screen.getByText("Deploy Test RGD")).toBeInTheDocument();
  });
});

// Note: Role-scoped namespace filtering tests removed — namespace access is now
// enforced server-side via Casbin namespace-scoped policies (roles[].destinations).
