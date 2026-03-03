import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { EnterpriseRequired } from "./EnterpriseRequired";

describe("EnterpriseRequired", () => {
  it("renders with default props", () => {
    render(<EnterpriseRequired />);

    expect(screen.getByText("Enterprise Feature")).toBeInTheDocument();
    expect(
      screen.getByText("This feature requires an Enterprise license")
    ).toBeInTheDocument();
  });

  it("renders with custom feature name", () => {
    render(<EnterpriseRequired feature="Policy Compliance" />);

    expect(
      screen.getByText("Policy Compliance requires an Enterprise license")
    ).toBeInTheDocument();
  });

  it("renders with custom description", () => {
    render(
      <EnterpriseRequired
        feature="Advanced Analytics"
        description="Get detailed insights into your Kubernetes deployments"
      />
    );

    expect(
      screen.getByText("Get detailed insights into your Kubernetes deployments")
    ).toBeInTheDocument();
  });

  it("displays enterprise features list", () => {
    render(<EnterpriseRequired />);

    expect(screen.getByText("Enterprise includes:")).toBeInTheDocument();
    expect(
      screen.getByText("OPA Gatekeeper policy compliance monitoring")
    ).toBeInTheDocument();
    expect(
      screen.getByText("Real-time violation tracking and alerts")
    ).toBeInTheDocument();
    expect(
      screen.getByText("Advanced RBAC and audit logging")
    ).toBeInTheDocument();
    expect(
      screen.getByText("Priority support and SLA guarantees")
    ).toBeInTheDocument();
  });

  it("displays Learn More button with correct link", () => {
    render(<EnterpriseRequired />);

    const learnMoreLink = screen.getByRole("link", {
      name: /learn more about enterprise/i,
    });
    expect(learnMoreLink).toHaveAttribute("href", "https://provops.dev/enterprise");
    expect(learnMoreLink).toHaveAttribute("target", "_blank");
    expect(learnMoreLink).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("displays Contact Sales button with mailto link", () => {
    render(<EnterpriseRequired />);

    const contactLink = screen.getByRole("link", { name: /contact sales/i });
    expect(contactLink).toHaveAttribute("href", "mailto:sales@provops.dev");
  });

  it("renders ShieldOff icon", () => {
    const { container } = render(<EnterpriseRequired />);

    // Look for the lucide icon by its class or structure
    const iconContainer = container.querySelector(".bg-amber-100");
    expect(iconContainer).toBeInTheDocument();
  });

  it("renders in a centered card layout", () => {
    const { container } = render(<EnterpriseRequired />);

    // Check for centering classes
    const wrapper = container.firstChild;
    expect(wrapper).toHaveClass("flex", "items-center", "justify-center");
  });

  it("does not render description when not provided", () => {
    render(<EnterpriseRequired feature="Test Feature" />);

    // The description paragraph should not exist
    const descriptions = screen.queryAllByText(
      /Get detailed insights|Monitor OPA/
    );
    expect(descriptions).toHaveLength(0);
  });
});
