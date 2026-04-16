// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Repository configuration form with ArgoCD-style authentication
 * Supports SSH, HTTPS, and GitHub App authentication methods
 */
import { useCallback, useMemo } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { DialogFooter } from "@/components/ui/dialog";
import { Loader2 } from "@/lib/icons";
import type {
  RepositoryConfig,
  AuthType,
  GitHubAppType,
  CreateRepositoryRequest,
  TestConnectionRequest,
  TestConnectionResponse,
} from "@/types/repository";
import type { Project } from "@/types/project";
import { useTestConnection } from "./hooks/useTestConnection";
import { useAuthTypeHandlers } from "./hooks/useAuthTypeHandlers";
import { usePasswordVisibilityToggles } from "./hooks/usePasswordVisibilityToggles";
import { buildAuthPayload } from "./hooks/buildAuthPayload";
import { SSHAuthSection } from "./SSHAuthSection";
import { HTTPSAuthSection } from "./HTTPSAuthSection";
import { GitHubAppAuthSection } from "./GitHubAppAuthSection";
import { TestConnectionButton } from "./TestConnectionButton";

// Base repository schema (common fields)
const baseSchema = {
  name: z.string().min(3, "Name must be at least 3 characters").max(100, "Name must not exceed 100 characters"),
  projectId: z.string().min(1, "Project is required"),
  repoURL: z.string().url("Must be a valid URL"),
  authType: z.enum(["ssh", "https", "github-app"] as const),
  defaultBranch: z.string().min(1, "Default branch is required"),
};

// Full repository schema with conditional auth validation
const repositorySchema = z.object({
  ...baseSchema,
  sshAuth: z.object({
    privateKey: z.string(),
  }).optional(),
  httpsAuth: z.object({
    username: z.string(),
    password: z.string(),
    bearerToken: z.string(),
    tlsClientCert: z.string(),
    tlsClientKey: z.string(),
    insecureSkipTLSVerify: z.boolean(),
  }).optional(),
  githubAppAuth: z.object({
    appType: z.enum(["github", "github-enterprise"] as const),
    appId: z.string(),
    installationId: z.string(),
    privateKey: z.string(),
    enterpriseUrl: z.string(),
  }).optional(),
}).superRefine((data, ctx) => {
  // Validate URL format based on auth type
  if (data.authType === "ssh") {
    if (!data.repoURL.startsWith("git@") && !data.repoURL.startsWith("ssh://")) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "SSH URLs must start with 'git@' or 'ssh://'",
        path: ["repoURL"],
      });
    }
    if (!data.sshAuth?.privateKey) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "Private key is required for SSH authentication",
        path: ["sshAuth", "privateKey"],
      });
    }
  } else if (data.authType === "https" || data.authType === "github-app") {
    if (!data.repoURL.startsWith("https://")) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "HTTPS URLs must start with 'https://'",
        path: ["repoURL"],
      });
    }

    if (data.authType === "https") {
      const hasAuth = data.httpsAuth?.username || data.httpsAuth?.bearerToken || data.httpsAuth?.tlsClientCert;
      if (!hasAuth) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: "At least one authentication method is required",
          path: ["httpsAuth"],
        });
      }
    }

    if (data.authType === "github-app") {
      if (!data.githubAppAuth?.appId) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: "App ID is required",
          path: ["githubAppAuth", "appId"],
        });
      }
      if (!data.githubAppAuth?.installationId) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: "Installation ID is required",
          path: ["githubAppAuth", "installationId"],
        });
      }
      if (!data.githubAppAuth?.privateKey) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: "Private key is required",
          path: ["githubAppAuth", "privateKey"],
        });
      }
      if (data.githubAppAuth?.appType === "github-enterprise" && !data.githubAppAuth?.enterpriseUrl) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: "Enterprise URL is required for GitHub Enterprise",
          path: ["githubAppAuth", "enterpriseUrl"],
        });
      }
    }
  }
});

type RepositoryFormData = z.infer<typeof repositorySchema>;

interface RepositoryFormProps {
  initialData?: RepositoryConfig;
  projects: Project[];
  onSubmit: (data: CreateRepositoryRequest) => Promise<void>;
  onCancel: () => void;
  onTestConnection?: (data: TestConnectionRequest) => Promise<TestConnectionResponse>;
  isLoading?: boolean;
}

export function RepositoryForm({
  initialData,
  projects,
  onSubmit,
  onCancel,
  onTestConnection,
  isLoading = false,
}: RepositoryFormProps) {
  const {
    register,
    handleSubmit,
    watch,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<RepositoryFormData>({
    resolver: zodResolver(repositorySchema),
    defaultValues: {
      name: initialData?.name || "",
      projectId: initialData?.projectId || "",
      repoURL: initialData?.repoURL || "",
      authType: (initialData?.authType as AuthType) || "https",
      defaultBranch: initialData?.defaultBranch || "main",
      sshAuth: {
        privateKey: "",
      },
      httpsAuth: {
        username: "",
        password: "",
        bearerToken: "",
        tlsClientCert: "",
        tlsClientKey: "",
        insecureSkipTLSVerify: false,
      },
      githubAppAuth: {
        appType: "github",
        appId: "",
        installationId: "",
        privateKey: "",
        enterpriseUrl: "",
      },
    },
  });

  // eslint-disable-next-line react-hooks/incompatible-library -- watch() is intentionally used for reactive form values
  const selectedAuthType = watch("authType");
  const githubAppType = watch("githubAppAuth.appType");

  const { testResult, isTesting, handleTestConnection } = useTestConnection(
    watch,
    selectedAuthType,
    onTestConnection
  );

  const { authTypeSetters, githubAppTypeSetters } = useAuthTypeHandlers(setValue);

  const {
    showPrivateKey, togglePrivateKey,
    showPassword, togglePassword,
    showBearerToken, toggleBearerToken,
  } = usePasswordVisibilityToggles();

  const onSubmitForm = useCallback(async (data: RepositoryFormData) => {
    const request: CreateRepositoryRequest = {
      name: data.name,
      projectId: data.projectId,
      repoURL: data.repoURL,
      authType: data.authType,
      defaultBranch: data.defaultBranch,
      ...buildAuthPayload(data),
    };

    await onSubmit(request);
  }, [onSubmit]);

  const projectOptions = useMemo(() => projects.map((project) => (
    <option key={project.name} value={project.name}>
      {project.name}
      {project.description && ` - ${project.description}`}
    </option>
  )), [projects]);

  const repoUrlPlaceholder = selectedAuthType === "ssh"
    ? "git@github.com:owner/repo.git"
    : "https://github.com/owner/repo.git";

  const repoUrlHint = selectedAuthType === "ssh"
    ? "SSH format: git@github.com:owner/repo.git"
    : "HTTPS format: https://github.com/owner/repo.git";

  const submitButtonLabel = initialData ? "Update Repository" : "Add Repository";

  const inputClasses = "w-full px-3 py-2 border border-border rounded-md bg-background focus:outline-none focus:ring-2 focus:ring-primary";
  const labelClasses = "block text-sm font-medium mb-1.5";
  const errorClasses = "mt-1 text-sm text-destructive";

  return (
    <form onSubmit={handleSubmit(onSubmitForm)} className="space-y-6">
      {/* Name */}
      <div>
        <label htmlFor="name" className={labelClasses}>
          Display Name *
        </label>
        <input
          id="name"
          {...register("name")}
          className={inputClasses}
          placeholder="My Production Repo"
        />
        {errors.name && <p className={errorClasses}>{errors.name.message}</p>}
      </div>

      {/* Project Selection */}
      <div>
        <label htmlFor="projectId" className={labelClasses}>
          Project *
        </label>
        <select id="projectId" {...register("projectId")} className={inputClasses}>
          <option value="">Select a project...</option>
          {projectOptions}
        </select>
        {errors.projectId && <p className={errorClasses}>{errors.projectId.message}</p>}
        <p className="mt-1 text-xs text-muted-foreground">
          The project determines which teams can access this repository
        </p>
      </div>

      {/* Repository URL */}
      <div>
        <label htmlFor="repoURL" className={labelClasses}>
          Repository URL *
        </label>
        <input
          id="repoURL"
          {...register("repoURL")}
          className={inputClasses}
          placeholder={repoUrlPlaceholder}
        />
        {errors.repoURL && <p className={errorClasses}>{errors.repoURL.message}</p>}
        <p className="mt-1 text-xs text-muted-foreground">
          {repoUrlHint}
        </p>
      </div>

      {/* Default Branch */}
      <div>
        <label htmlFor="defaultBranch" className={labelClasses}>
          Default Branch *
        </label>
        <input
          id="defaultBranch"
          {...register("defaultBranch")}
          className={inputClasses}
          placeholder="main"
        />
        {errors.defaultBranch && <p className={errorClasses}>{errors.defaultBranch.message}</p>}
      </div>

      {/* Authentication Type Selector */}
      <div>
        {/* eslint-disable-next-line jsx-a11y/label-has-associated-control */}
        <label className={labelClasses} id="auth-method-label">Authentication Method *</label>
        <div className="flex gap-2 mt-2">
          {(["ssh", "https", "github-app"] as AuthType[]).map((type) => (
            <button
              key={type}
              type="button"
              onClick={authTypeSetters[type]}
              className={`flex-1 px-4 py-2 text-sm font-medium rounded-md border transition-colors ${
                selectedAuthType === type
                  ? "bg-primary text-primary-foreground border-primary"
                  : "bg-background border-border hover:bg-accent hover:text-accent-foreground"
              }`}
            >
              {type === "ssh" && "SSH"}
              {type === "https" && "HTTPS"}
              {type === "github-app" && "GitHub App"}
            </button>
          ))}
        </div>
      </div>

      {/* SSH Authentication Fields */}
      {selectedAuthType === "ssh" && (
        <SSHAuthSection
          registerPrivateKey={register("sshAuth.privateKey")}
          privateKeyError={errors.sshAuth?.privateKey?.message}
          showPrivateKey={showPrivateKey}
          togglePrivateKey={togglePrivateKey}
        />
      )}

      {/* HTTPS Authentication Fields */}
      {selectedAuthType === "https" && (
        <HTTPSAuthSection
          registerUsername={register("httpsAuth.username")}
          registerPassword={register("httpsAuth.password")}
          registerBearerToken={register("httpsAuth.bearerToken")}
          registerTlsCert={register("httpsAuth.tlsClientCert")}
          registerTlsKey={register("httpsAuth.tlsClientKey")}
          httpsAuthError={
            errors.httpsAuth && typeof errors.httpsAuth === "object" && "message" in errors.httpsAuth
              ? (errors.httpsAuth as { message?: string }).message
              : undefined
          }
          showPassword={showPassword}
          togglePassword={togglePassword}
          showBearerToken={showBearerToken}
          toggleBearerToken={toggleBearerToken}
        />
      )}

      {/* GitHub App Authentication Fields */}
      {selectedAuthType === "github-app" && (
        <GitHubAppAuthSection
          registerAppId={register("githubAppAuth.appId")}
          registerInstallationId={register("githubAppAuth.installationId")}
          registerPrivateKey={register("githubAppAuth.privateKey")}
          registerEnterpriseUrl={register("githubAppAuth.enterpriseUrl")}
          appIdError={errors.githubAppAuth?.appId?.message}
          installationIdError={errors.githubAppAuth?.installationId?.message}
          privateKeyError={errors.githubAppAuth?.privateKey?.message}
          enterpriseUrlError={errors.githubAppAuth?.enterpriseUrl?.message}
          githubAppType={githubAppType as GitHubAppType}
          githubAppTypeSetters={githubAppTypeSetters}
          showPrivateKey={showPrivateKey}
          togglePrivateKey={togglePrivateKey}
        />
      )}

      {/* Test Connection Button */}
      {onTestConnection && (
        <TestConnectionButton
          onTest={handleTestConnection}
          isTesting={isTesting}
          isDisabled={!watch("repoURL")}
          testResult={testResult}
        />
      )}

      {/* Form Actions */}
      <DialogFooter>
        <Button type="button" variant="outline" onClick={onCancel} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button type="submit" disabled={isSubmitting || isLoading}>
          {isSubmitting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
          {submitButtonLabel}
        </Button>
      </DialogFooter>
    </form>
  );
}
