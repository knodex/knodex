import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { RepositoryForm } from "./RepositoryForm";
import type { Project } from "@/types/project";
import type {
  TestConnectionResponse,
} from "@/types/repository";

const mockProjects: Project[] = [
  {
    name: "project-a",
    description: "Project A Description",
    resourceVersion: "1",
    createdAt: "2024-01-01T00:00:00Z",
  },
  {
    name: "project-b",
    description: "Project B Description",
    resourceVersion: "1",
    createdAt: "2024-01-01T00:00:00Z",
  },
];

describe("RepositoryForm", () => {
  const mockOnSubmit = vi.fn();
  const mockOnCancel = vi.fn();
  const mockOnTestConnection = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("basic rendering", () => {
    it("renders all required form fields", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      expect(screen.getByLabelText("Display Name *")).toBeInTheDocument();
      expect(screen.getByLabelText("Project *")).toBeInTheDocument();
      expect(screen.getByLabelText("Repository URL *")).toBeInTheDocument();
      expect(screen.getByLabelText("Default Branch *")).toBeInTheDocument();
      // Authentication Method uses buttons instead of a form control
      expect(screen.getByText("Authentication Method *")).toBeInTheDocument();
    });

    it("renders authentication type buttons", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      expect(screen.getByText("SSH")).toBeInTheDocument();
      expect(screen.getByText("HTTPS")).toBeInTheDocument();
      expect(screen.getByText("GitHub App")).toBeInTheDocument();
    });

    it("renders form action buttons", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      expect(
        screen.getByRole("button", { name: /add repository/i })
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /cancel/i })
      ).toBeInTheDocument();
    });

  });

  describe("project selection", () => {
    it("renders project dropdown with all projects", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      const select = screen.getByLabelText("Project *") as HTMLSelectElement;
      expect(select).toBeInTheDocument();

      // Check for empty option
      const options = Array.from(select.options);
      expect(options[0].value).toBe("");
      expect(options[0].text).toBe("Select a project...");

      // Check for project options
      expect(options[1].value).toBe("project-a");
      expect(options[1].text).toContain("project-a");
      expect(options[1].text).toContain("Project A Description");

      expect(options[2].value).toBe("project-b");
      expect(options[2].text).toContain("project-b");
    });

    it("displays helper text for project selection", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      expect(
        screen.getByText(
          "The project determines which teams can access this repository"
        )
      ).toBeInTheDocument();
    });
  });

  describe("authentication type switching", () => {
    it("defaults to HTTPS authentication", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      const httpsButton = screen.getByRole("button", { name: "HTTPS" });
      expect(httpsButton).toHaveClass("bg-primary");
    });

    it("switches to SSH authentication", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      const sshButton = screen.getByRole("button", { name: "SSH" });
      fireEvent.click(sshButton);

      expect(sshButton).toHaveClass("bg-primary");
      expect(screen.getByLabelText("SSH Private Key *")).toBeInTheDocument();
    });

    it("switches to GitHub App authentication", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      const githubAppButton = screen.getByRole("button", {
        name: "GitHub App",
      });
      fireEvent.click(githubAppButton);

      expect(githubAppButton).toHaveClass("bg-primary");
      // GitHub App Type uses buttons instead of a form control
      expect(screen.getByText("GitHub App Type *")).toBeInTheDocument();
    });
  });

  describe("SSH authentication fields", () => {
    it("renders SSH fields when SSH is selected", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      const sshButton = screen.getByRole("button", { name: "SSH" });
      fireEvent.click(sshButton);

      expect(
        screen.getByText("SSH authentication using a private key")
      ).toBeInTheDocument();
      expect(screen.getByLabelText("SSH Private Key *")).toBeInTheDocument();
      expect(
        screen.getByText("Paste your SSH private key in PEM format")
      ).toBeInTheDocument();
    });

    it("updates placeholder text for SSH URLs", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      const sshButton = screen.getByRole("button", { name: "SSH" });
      fireEvent.click(sshButton);

      const urlInput = screen.getByLabelText("Repository URL *");
      expect(urlInput).toHaveAttribute(
        "placeholder",
        "git@github.com:owner/repo.git"
      );
      expect(
        screen.getByText("SSH format: git@github.com:owner/repo.git")
      ).toBeInTheDocument();
    });

    it("toggles SSH private key visibility", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      const sshButton = screen.getByRole("button", { name: "SSH" });
      fireEvent.click(sshButton);

      const privateKeyField = screen.getByLabelText(
        "SSH Private Key *"
      ) as HTMLTextAreaElement;
      expect(privateKeyField).toHaveAttribute("placeholder", "••••••••••••");

      // Find the eye toggle button - it's in the same parent container as the label
      const sshLabel = screen.getByText("SSH Private Key *");
      const labelContainer = sshLabel.closest("div");
      const eyeToggle = labelContainer?.querySelector("button");
      expect(eyeToggle).toBeDefined();

      if (eyeToggle) {
        fireEvent.click(eyeToggle);
        expect(privateKeyField.placeholder).toContain("BEGIN OPENSSH");
      }
    });
  });

  describe("HTTPS authentication fields", () => {
    it("renders HTTPS fields when HTTPS is selected", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      expect(
        screen.getByText(
          "HTTPS authentication - provide at least one method"
        )
      ).toBeInTheDocument();
      expect(screen.getByLabelText("Username")).toBeInTheDocument();
      expect(
        screen.getByLabelText("Password / Token")
      ).toBeInTheDocument();
      expect(screen.getByLabelText("Bearer Token")).toBeInTheDocument();
    });

    it("updates placeholder text for HTTPS URLs", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      const urlInput = screen.getByLabelText("Repository URL *");
      expect(urlInput).toHaveAttribute(
        "placeholder",
        "https://github.com/owner/repo.git"
      );
      expect(
        screen.getByText("HTTPS format: https://github.com/owner/repo.git")
      ).toBeInTheDocument();
    });

    it("toggles password visibility", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      const passwordField = screen.getByLabelText(
        "Password / Token"
      ) as HTMLInputElement;
      expect(passwordField.type).toBe("password");

      const toggleButtons = screen.getAllByRole("button");
      const passwordToggle = toggleButtons.find(
        (btn) =>
          btn.closest("div")?.querySelector("#httpsPassword") !== null &&
          btn.getAttribute("type") === "button" &&
          btn.textContent === ""
      );

      if (passwordToggle) {
        fireEvent.click(passwordToggle);
        expect(passwordField.type).toBe("text");
      }
    });

    it("toggles bearer token visibility", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      const bearerTokenField = screen.getByLabelText(
        "Bearer Token"
      ) as HTMLInputElement;
      expect(bearerTokenField.type).toBe("password");

      const toggleButtons = screen.getAllByRole("button");
      const tokenToggle = toggleButtons.find(
        (btn) =>
          btn.closest("div")?.querySelector("#httpsBearerToken") !== null &&
          btn.getAttribute("type") === "button" &&
          btn.textContent === ""
      );

      if (tokenToggle) {
        fireEvent.click(tokenToggle);
        expect(bearerTokenField.type).toBe("text");
      }
    });

    it("renders TLS advanced fields in collapsible section", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      const detailsElement = screen.getByText(
        "TLS Client Certificate (Advanced)"
      ).closest("details");
      expect(detailsElement).toBeInTheDocument();

      // Initially collapsed - fields should not be visible
      expect(
        screen.queryByLabelText("TLS Client Certificate")
      ).not.toBeVisible();

      // Click to expand
      fireEvent.click(screen.getByText("TLS Client Certificate (Advanced)"));

      // Now fields should be visible
      expect(screen.getByLabelText("TLS Client Certificate")).toBeVisible();
      expect(screen.getByLabelText("TLS Client Key")).toBeVisible();
    });
  });

  describe("GitHub App authentication fields", () => {
    it("renders GitHub App fields when selected", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      const githubAppButton = screen.getByRole("button", {
        name: "GitHub App",
      });
      fireEvent.click(githubAppButton);

      expect(
        screen.getByText(
          "GitHub App authentication - recommended for organizations"
        )
      ).toBeInTheDocument();
      // GitHub App Type uses buttons instead of a form control
      expect(screen.getByText("GitHub App Type *")).toBeInTheDocument();
      expect(screen.getByLabelText("App ID *")).toBeInTheDocument();
      expect(screen.getByLabelText("Installation ID *")).toBeInTheDocument();
      expect(screen.getByLabelText("Private Key *")).toBeInTheDocument();
    });

    it("defaults to GitHub.com app type", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      const githubAppButton = screen.getByRole("button", {
        name: "GitHub App",
      });
      fireEvent.click(githubAppButton);

      const githubComButton = screen.getByRole("button", {
        name: "GitHub.com",
      });
      expect(githubComButton).toHaveClass("bg-primary");
    });

    it("shows enterprise URL field when GitHub Enterprise is selected", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      const githubAppButton = screen.getByRole("button", {
        name: "GitHub App",
      });
      fireEvent.click(githubAppButton);

      // Initially no enterprise URL field
      expect(
        screen.queryByLabelText("Enterprise URL *")
      ).not.toBeInTheDocument();

      // Switch to GitHub Enterprise
      const enterpriseButton = screen.getByRole("button", {
        name: "GitHub Enterprise",
      });
      fireEvent.click(enterpriseButton);

      // Now enterprise URL field should appear
      expect(screen.getByLabelText("Enterprise URL *")).toBeInTheDocument();
    });

    it("toggles GitHub App private key visibility", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      const githubAppButton = screen.getByRole("button", {
        name: "GitHub App",
      });
      fireEvent.click(githubAppButton);

      const privateKeyField = screen.getByLabelText(
        "Private Key *"
      ) as HTMLTextAreaElement;
      expect(privateKeyField).toHaveAttribute("placeholder", "••••••••••••");

      const toggleButtons = screen.getAllByRole("button");
      const eyeToggle = toggleButtons.find(
        (btn) =>
          btn.closest("div")?.querySelector("#githubAppPrivateKey") !== null &&
          btn !== githubAppButton &&
          btn.getAttribute("type") === "button" &&
          btn.textContent === ""
      );

      if (eyeToggle) {
        fireEvent.click(eyeToggle);
        expect(privateKeyField.placeholder).toContain("BEGIN RSA");
      }
    });
  });

  describe("test connection", () => {
    it("renders test connection button when onTestConnection provided", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
          onTestConnection={mockOnTestConnection}
        />
      );

      expect(
        screen.getByRole("button", { name: /test connection/i })
      ).toBeInTheDocument();
    });

    it("does not render test connection button when onTestConnection not provided", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      expect(
        screen.queryByRole("button", { name: /test connection/i })
      ).not.toBeInTheDocument();
    });

    it("disables test button when repoURL is empty", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
          onTestConnection={mockOnTestConnection}
        />
      );

      const testButton = screen.getByRole("button", {
        name: /test connection/i,
      });
      expect(testButton).toBeDisabled();
    });

    it("calls onTestConnection with correct HTTPS data", async () => {
      const mockTestResult: TestConnectionResponse = {
        valid: true,
        message: "Connection successful",
      };
      mockOnTestConnection.mockResolvedValue(mockTestResult);

      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
          onTestConnection={mockOnTestConnection}
        />
      );

      // Fill in form
      fireEvent.change(screen.getByLabelText("Repository URL *"), {
        target: { value: "https://github.com/owner/repo.git" },
      });
      fireEvent.change(screen.getByLabelText("Username"), {
        target: { value: "testuser" },
      });
      fireEvent.change(screen.getByLabelText("Password / Token"), {
        target: { value: "testpass" },
      });

      const testButton = screen.getByRole("button", {
        name: /test connection/i,
      });
      expect(testButton).not.toBeDisabled();

      fireEvent.click(testButton);

      await waitFor(() => {
        expect(mockOnTestConnection).toHaveBeenCalledWith({
          repoURL: "https://github.com/owner/repo.git",
          authType: "https",
          httpsAuth: {
            username: "testuser",
            password: "testpass",
            bearerToken: undefined,
            tlsClientCert: undefined,
            tlsClientKey: undefined,
            insecureSkipTLSVerify: undefined,
          },
        });
      });
    });

    it("displays success message after successful test", async () => {
      const mockTestResult: TestConnectionResponse = {
        valid: true,
        message: "Connection successful",
      };
      mockOnTestConnection.mockResolvedValue(mockTestResult);

      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
          onTestConnection={mockOnTestConnection}
        />
      );

      fireEvent.change(screen.getByLabelText("Repository URL *"), {
        target: { value: "https://github.com/owner/repo.git" },
      });

      fireEvent.click(
        screen.getByRole("button", { name: /test connection/i })
      );

      await waitFor(() => {
        expect(screen.getByText("Connection successful")).toBeInTheDocument();
      });
    });

    it("displays error message after failed test", async () => {
      const mockTestResult: TestConnectionResponse = {
        valid: false,
        message: "Authentication failed",
      };
      mockOnTestConnection.mockResolvedValue(mockTestResult);

      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
          onTestConnection={mockOnTestConnection}
        />
      );

      fireEvent.change(screen.getByLabelText("Repository URL *"), {
        target: { value: "https://github.com/owner/repo.git" },
      });

      fireEvent.click(
        screen.getByRole("button", { name: /test connection/i })
      );

      await waitFor(() => {
        expect(screen.getByText("Authentication failed")).toBeInTheDocument();
      });
    });

    it("shows loading state during test", async () => {
      mockOnTestConnection.mockImplementation(
        () =>
          new Promise((resolve) =>
            setTimeout(() => resolve({ valid: true, message: "Success" }), 100)
          )
      );

      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
          onTestConnection={mockOnTestConnection}
        />
      );

      fireEvent.change(screen.getByLabelText("Repository URL *"), {
        target: { value: "https://github.com/owner/repo.git" },
      });

      fireEvent.click(
        screen.getByRole("button", { name: /test connection/i })
      );

      expect(
        screen.getByRole("button", { name: /testing connection/i })
      ).toBeInTheDocument();

      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: /test connection/i })
        ).toBeInTheDocument();
      });
    });
  });

  describe("form submission", () => {
    it("calls onSubmit with correct HTTPS data", async () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      // Fill in form
      fireEvent.change(screen.getByLabelText("Display Name *"), {
        target: { value: "Test Repo" },
      });
      fireEvent.change(screen.getByLabelText("Project *"), {
        target: { value: "project-a" },
      });
      fireEvent.change(screen.getByLabelText("Repository URL *"), {
        target: { value: "https://github.com/owner/repo.git" },
      });
      fireEvent.change(screen.getByLabelText("Default Branch *"), {
        target: { value: "main" },
      });
      fireEvent.change(screen.getByLabelText("Username"), {
        target: { value: "testuser" },
      });
      fireEvent.change(screen.getByLabelText("Password / Token"), {
        target: { value: "testpass" },
      });

      fireEvent.click(
        screen.getByRole("button", { name: /add repository/i })
      );

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith({
          name: "Test Repo",
          projectId: "project-a",
          repoURL: "https://github.com/owner/repo.git",
          authType: "https",
          defaultBranch: "main",
          httpsAuth: {
            username: "testuser",
            password: "testpass",
            bearerToken: undefined,
            tlsClientCert: undefined,
            tlsClientKey: undefined,
            insecureSkipTLSVerify: undefined,
          },
        });
      });
    });

    it("calls onSubmit with correct SSH data", async () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      // Switch to SSH
      fireEvent.click(screen.getByRole("button", { name: "SSH" }));

      // Fill in form - use ssh:// URL format that passes Zod's .url() validation
      fireEvent.change(screen.getByLabelText("Display Name *"), {
        target: { value: "Test SSH Repo" },
      });
      fireEvent.change(screen.getByLabelText("Project *"), {
        target: { value: "project-b" },
      });
      fireEvent.change(screen.getByLabelText("Repository URL *"), {
        target: { value: "ssh://git@github.com/owner/repo.git" },
      });
      fireEvent.change(screen.getByLabelText("Default Branch *"), {
        target: { value: "develop" },
      });
      fireEvent.change(screen.getByLabelText("SSH Private Key *"), {
        target: { value: "-----BEGIN OPENSSH PRIVATE KEY-----\nkey\n-----END OPENSSH PRIVATE KEY-----" },
      });

      fireEvent.click(
        screen.getByRole("button", { name: /add repository/i })
      );

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith({
          name: "Test SSH Repo",
          projectId: "project-b",
          repoURL: "ssh://git@github.com/owner/repo.git",
          authType: "ssh",
          defaultBranch: "develop",
          sshAuth: {
            privateKey: "-----BEGIN OPENSSH PRIVATE KEY-----\nkey\n-----END OPENSSH PRIVATE KEY-----",
          },
        });
      });
    });

    it("disables submit button during submission", async () => {
      mockOnSubmit.mockImplementation(
        () => new Promise((resolve) => setTimeout(resolve, 100))
      );

      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      // Fill in minimum required fields
      fireEvent.change(screen.getByLabelText("Display Name *"), {
        target: { value: "Test" },
      });
      fireEvent.change(screen.getByLabelText("Project *"), {
        target: { value: "project-a" },
      });
      fireEvent.change(screen.getByLabelText("Repository URL *"), {
        target: { value: "https://github.com/test/test.git" },
      });
      fireEvent.change(screen.getByLabelText("Username"), {
        target: { value: "user" },
      });

      const submitButton = screen.getByRole("button", {
        name: /add repository/i,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(submitButton).toBeDisabled();
      });
    });
  });

  describe("cancel action", () => {
    it("calls onCancel when cancel button is clicked", () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      fireEvent.click(screen.getByRole("button", { name: /cancel/i }));

      expect(mockOnCancel).toHaveBeenCalledTimes(1);
    });
  });

  describe("edit mode", () => {
    it("populates form fields with initial data", () => {
      const initialData = {
        id: "repo-1",
        name: "Existing Repo",
        projectId: "project-a",
        repoURL: "https://github.com/existing/repo.git",
        authType: "https" as const,
        defaultBranch: "develop",
        resourceVersion: "1",
        createdAt: "2024-01-01T00:00:00Z",
      };

      render(
        <RepositoryForm
          initialData={initialData}
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      expect(screen.getByLabelText("Display Name *")).toHaveValue(
        "Existing Repo"
      );
      expect(screen.getByLabelText("Project *")).toHaveValue("project-a");
      expect(screen.getByLabelText("Repository URL *")).toHaveValue(
        "https://github.com/existing/repo.git"
      );
      expect(screen.getByLabelText("Default Branch *")).toHaveValue("develop");
    });

    it("shows Update Repository button in edit mode", () => {
      const initialData = {
        id: "repo-1",
        name: "Existing Repo",
        projectId: "project-a",
        repoURL: "https://github.com/existing/repo.git",
        authType: "https" as const,
        defaultBranch: "main",
        resourceVersion: "1",
        createdAt: "2024-01-01T00:00:00Z",
      };

      render(
        <RepositoryForm
          initialData={initialData}
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      expect(
        screen.getByRole("button", { name: /update repository/i })
      ).toBeInTheDocument();
    });
  });

  describe("form validation", () => {
    it("shows validation error for empty name", async () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      fireEvent.change(screen.getByLabelText("Display Name *"), {
        target: { value: "ab" },
      });
      fireEvent.blur(screen.getByLabelText("Display Name *"));

      fireEvent.click(
        screen.getByRole("button", { name: /add repository/i })
      );

      await waitFor(() => {
        expect(
          screen.getByText(/name must be at least 3 characters/i)
        ).toBeInTheDocument();
      });
    });

    it("shows validation error for missing project", async () => {
      render(
        <RepositoryForm
          projects={mockProjects}
          onSubmit={mockOnSubmit}
          onCancel={mockOnCancel}
        />
      );

      fireEvent.change(screen.getByLabelText("Display Name *"), {
        target: { value: "Test Repo" },
      });
      fireEvent.change(screen.getByLabelText("Repository URL *"), {
        target: { value: "https://github.com/test/test.git" },
      });

      fireEvent.click(
        screen.getByRole("button", { name: /add repository/i })
      );

      await waitFor(() => {
        expect(
          screen.getByText(/project is required/i)
        ).toBeInTheDocument();
      });
    });
  });
});
