import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { DeploymentModeSelector } from "./DeploymentModeSelector";
import type { RepositoryConfig } from "@/types/repository";

const mockRepositories: RepositoryConfig[] = [
  {
    id: "repo-1",
    name: "Production Deployments",
    repoURL: "https://github.com/my-org/prod-manifests",
    defaultBranch: "main",
    authType: "ssh",
  },
  {
    id: "repo-2",
    name: "Development Deployments",
    repoURL: "https://github.com/my-org/dev-manifests",
    defaultBranch: "develop",
    authType: "token",
  },
];

describe("DeploymentModeSelector", () => {
  const defaultProps = {
    mode: "direct" as const,
    onModeChange: vi.fn(),
    repositoryId: "",
    onRepositoryChange: vi.fn(),
    gitBranch: "",
    onGitBranchChange: vi.fn(),
    gitPath: "",
    onGitPathChange: vi.fn(),
    repositories: mockRepositories,
  };

  describe("Rendering", () => {
    it("renders all three deployment mode options", () => {
      render(<DeploymentModeSelector {...defaultProps} />);

      expect(screen.getByText("Direct")).toBeInTheDocument();
      expect(screen.getByText("GitOps")).toBeInTheDocument();
      expect(screen.getByText("Hybrid")).toBeInTheDocument();
    });

    it("renders the Deployment Mode label", () => {
      render(<DeploymentModeSelector {...defaultProps} />);

      expect(screen.getByText("Deployment Mode")).toBeInTheDocument();
    });

    it("shows mode description for selected mode", () => {
      render(<DeploymentModeSelector {...defaultProps} mode="direct" />);

      expect(
        screen.getByText(/Deploy directly to the Kubernetes cluster via API/)
      ).toBeInTheDocument();
    });

    it("applies custom className when provided", () => {
      const { container } = render(
        <DeploymentModeSelector {...defaultProps} className="custom-class" />
      );

      expect(container.firstChild).toHaveClass("custom-class");
    });
  });

  describe("Mode Selection", () => {
    it("highlights the selected mode", () => {
      render(<DeploymentModeSelector {...defaultProps} mode="gitops" />);

      // Find the GitOps button and check it has selected styling
      const gitopsButton = screen.getByRole("button", { name: /GitOps/i });
      expect(gitopsButton).toHaveClass("border-primary");
    });

    it("calls onModeChange when Direct mode is selected", () => {
      const onModeChange = vi.fn();
      render(
        <DeploymentModeSelector
          {...defaultProps}
          mode="gitops"
          onModeChange={onModeChange}
        />
      );

      const directButton = screen.getByRole("button", { name: /Direct/i });
      fireEvent.click(directButton);

      expect(onModeChange).toHaveBeenCalledWith("direct");
    });

    it("calls onModeChange when GitOps mode is selected", () => {
      const onModeChange = vi.fn();
      render(
        <DeploymentModeSelector {...defaultProps} onModeChange={onModeChange} />
      );

      const gitopsButton = screen.getByRole("button", { name: /GitOps/i });
      fireEvent.click(gitopsButton);

      expect(onModeChange).toHaveBeenCalledWith("gitops");
    });

    it("calls onModeChange when Hybrid mode is selected", () => {
      const onModeChange = vi.fn();
      render(
        <DeploymentModeSelector {...defaultProps} onModeChange={onModeChange} />
      );

      const hybridButton = screen.getByRole("button", { name: /Hybrid/i });
      fireEvent.click(hybridButton);

      expect(onModeChange).toHaveBeenCalledWith("hybrid");
    });
  });

  describe("Repository Selector", () => {
    it("does not show repository selector when Direct mode is selected", () => {
      render(<DeploymentModeSelector {...defaultProps} mode="direct" />);

      expect(screen.queryByLabelText(/Git Repository/i)).not.toBeInTheDocument();
    });

    it("shows repository selector when GitOps mode is selected", () => {
      render(<DeploymentModeSelector {...defaultProps} mode="gitops" />);

      expect(screen.getByText("Git Repository")).toBeInTheDocument();
    });

    it("shows repository selector when Hybrid mode is selected", () => {
      render(<DeploymentModeSelector {...defaultProps} mode="hybrid" />);

      expect(screen.getByText("Git Repository")).toBeInTheDocument();
    });

    it("displays repository options in dropdown", () => {
      render(<DeploymentModeSelector {...defaultProps} mode="gitops" />);

      expect(screen.getByText("Select a repository...")).toBeInTheDocument();
      expect(screen.getByText(/Production Deployments/)).toBeInTheDocument();
      expect(screen.getByText(/Development Deployments/)).toBeInTheDocument();
    });

    it("shows selected repository when repositoryId is provided", () => {
      render(
        <DeploymentModeSelector
          {...defaultProps}
          mode="gitops"
          repositoryId="repo-1"
        />
      );

      const select = screen.getByRole("combobox") as HTMLSelectElement;
      expect(select.value).toBe("repo-1");
    });

    it("calls onRepositoryChange when repository is selected", () => {
      const onRepositoryChange = vi.fn();
      render(
        <DeploymentModeSelector
          {...defaultProps}
          mode="gitops"
          onRepositoryChange={onRepositoryChange}
        />
      );

      const select = screen.getByRole("combobox");
      fireEvent.change(select, { target: { value: "repo-2" } });

      expect(onRepositoryChange).toHaveBeenCalledWith("repo-2");
    });

    it("shows repository details when selected", () => {
      render(
        <DeploymentModeSelector
          {...defaultProps}
          mode="gitops"
          repositoryId="repo-1"
        />
      );

      // Should show branch info for selected repository
      expect(screen.getByText(/main/)).toBeInTheDocument();
    });

    it("shows warning when no repository is selected", () => {
      render(
        <DeploymentModeSelector
          {...defaultProps}
          mode="gitops"
          repositoryId=""
        />
      );

      expect(
        screen.getByText(/Please select a repository/i)
      ).toBeInTheDocument();
    });
  });

  describe("Loading and Error States", () => {
    it("shows loading state when repositories are loading", () => {
      render(
        <DeploymentModeSelector
          {...defaultProps}
          mode="gitops"
          isLoadingRepositories={true}
          repositories={[]}
        />
      );

      expect(screen.getByText(/Loading repositories/i)).toBeInTheDocument();
    });

    it("shows error when repositories fail to load", () => {
      render(
        <DeploymentModeSelector
          {...defaultProps}
          mode="gitops"
          repositoriesError="Failed to load repositories"
          repositories={[]}
        />
      );

      expect(
        screen.getByText(/Failed to load repositories/i)
      ).toBeInTheDocument();
    });

    it("shows message when no repositories are configured", () => {
      render(
        <DeploymentModeSelector
          {...defaultProps}
          mode="gitops"
          repositories={[]}
        />
      );

      expect(
        screen.getByText(/No repositories configured/i)
      ).toBeInTheDocument();
    });
  });

  describe("Hybrid Mode", () => {
    it("shows hybrid mode explanation when hybrid is selected", () => {
      render(<DeploymentModeSelector {...defaultProps} mode="hybrid" />);

      expect(
        screen.getByText(/Manifest is applied to the cluster immediately/i)
      ).toBeInTheDocument();
      expect(
        screen.getByText(/Manifest is then pushed to Git asynchronously/i)
      ).toBeInTheDocument();
    });
  });

  describe("Allowed Modes Restriction", () => {
    it("shows all three modes when allowedModes is undefined", () => {
      render(<DeploymentModeSelector {...defaultProps} allowedModes={undefined} />);

      expect(screen.getByRole("button", { name: /Direct/i })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /GitOps/i })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /Hybrid/i })).toBeInTheDocument();
    });

    it("shows all three modes when allowedModes is empty array", () => {
      render(<DeploymentModeSelector {...defaultProps} allowedModes={[]} />);

      expect(screen.getByRole("button", { name: /Direct/i })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /GitOps/i })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /Hybrid/i })).toBeInTheDocument();
    });

    it("shows only gitops mode when allowedModes contains only gitops", () => {
      render(
        <DeploymentModeSelector
          {...defaultProps}
          mode="gitops"
          allowedModes={["gitops"]}
        />
      );

      expect(screen.queryByRole("button", { name: /^Direct$/i })).not.toBeInTheDocument();
      expect(screen.getByRole("button", { name: /GitOps/i })).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: /^Hybrid$/i })).not.toBeInTheDocument();
    });

    it("shows only direct and hybrid modes when allowedModes contains direct and hybrid", () => {
      render(
        <DeploymentModeSelector
          {...defaultProps}
          mode="direct"
          allowedModes={["direct", "hybrid"]}
        />
      );

      expect(screen.getByRole("button", { name: /Direct/i })).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: /GitOps/i })).not.toBeInTheDocument();
      expect(screen.getByRole("button", { name: /Hybrid/i })).toBeInTheDocument();
    });

    it("shows restriction banner when only one mode is allowed", () => {
      render(
        <DeploymentModeSelector
          {...defaultProps}
          mode="gitops"
          allowedModes={["gitops"]}
        />
      );

      // The banner text contains "This RGD only allows GitOps deployment mode"
      const banner = screen.getByText(/This RGD only allows/i);
      expect(banner).toBeInTheDocument();
      expect(banner.textContent).toContain("GitOps");
    });

    it("shows restriction banner with multiple allowed modes", () => {
      render(
        <DeploymentModeSelector
          {...defaultProps}
          mode="direct"
          allowedModes={["direct", "gitops"]}
        />
      );

      expect(
        screen.getByText(/This RGD is restricted to the following deployment modes/i)
      ).toBeInTheDocument();
    });

    it("does not show restriction banner when all modes are allowed", () => {
      render(
        <DeploymentModeSelector
          {...defaultProps}
          allowedModes={["direct", "gitops", "hybrid"]}
        />
      );

      expect(
        screen.queryByText(/This RGD only allows/i)
      ).not.toBeInTheDocument();
      expect(
        screen.queryByText(/This RGD is restricted/i)
      ).not.toBeInTheDocument();

      // Verify all buttons are enabled (not disabled) when all modes allowed
      expect(screen.getByRole("button", { name: /Direct/i })).not.toBeDisabled();
      expect(screen.getByRole("button", { name: /GitOps/i })).not.toBeDisabled();
      expect(screen.getByRole("button", { name: /Hybrid/i })).not.toBeDisabled();
    });

    it("disables the button when only one mode is allowed", () => {
      render(
        <DeploymentModeSelector
          {...defaultProps}
          mode="gitops"
          allowedModes={["gitops"]}
        />
      );

      const gitopsButton = screen.getByRole("button", { name: /GitOps/i });
      expect(gitopsButton).toBeDisabled();
    });

    it("auto-selects single allowed mode when current mode is not allowed", () => {
      const onModeChange = vi.fn();
      render(
        <DeploymentModeSelector
          {...defaultProps}
          mode="direct" // Current mode not in allowed list
          allowedModes={["gitops"]}
          onModeChange={onModeChange}
        />
      );

      // Should auto-switch to the only allowed mode
      expect(onModeChange).toHaveBeenCalledWith("gitops");
    });

    it("auto-selects first allowed mode when current mode is not in list", () => {
      const onModeChange = vi.fn();
      render(
        <DeploymentModeSelector
          {...defaultProps}
          mode="hybrid" // Current mode not in allowed list
          allowedModes={["direct", "gitops"]}
          onModeChange={onModeChange}
        />
      );

      // Should auto-switch to first allowed mode
      expect(onModeChange).toHaveBeenCalledWith("direct");
    });

    it("does not auto-switch when current mode is in allowed list", () => {
      const onModeChange = vi.fn();
      render(
        <DeploymentModeSelector
          {...defaultProps}
          mode="gitops"
          allowedModes={["direct", "gitops"]}
          onModeChange={onModeChange}
        />
      );

      // Should not switch modes since gitops is allowed
      expect(onModeChange).not.toHaveBeenCalled();
    });

    it("uses grid-cols-2 when two modes are allowed", () => {
      const { container } = render(
        <DeploymentModeSelector
          {...defaultProps}
          mode="direct"
          allowedModes={["direct", "gitops"]}
        />
      );

      const grid = container.querySelector(".grid-cols-2");
      expect(grid).toBeInTheDocument();
    });

    it("uses grid-cols-1 when one mode is allowed", () => {
      const { container } = render(
        <DeploymentModeSelector
          {...defaultProps}
          mode="gitops"
          allowedModes={["gitops"]}
        />
      );

      const grid = container.querySelector(".grid-cols-1");
      expect(grid).toBeInTheDocument();
    });
  });
});
