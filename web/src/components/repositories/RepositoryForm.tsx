// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Repository configuration form with ArgoCD-style authentication
 * Supports SSH, HTTPS, and GitHub App authentication methods
 */
import { useState, useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { AlertCircle, CheckCircle2, Loader2, Info, Eye, EyeOff } from "lucide-react";
import type {
  RepositoryConfig,
  AuthType,
  GitHubAppType,
  CreateRepositoryRequest,
  TestConnectionRequest,
  TestConnectionResponse,
} from "@/types/repository";
import type { Project } from "@/types/project";

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
  const [testResult, setTestResult] = useState<TestConnectionResponse | null>(null);
  const [isTesting, setIsTesting] = useState(false);
  const [showPrivateKey, setShowPrivateKey] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [showBearerToken, setShowBearerToken] = useState(false);

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

  const selectedAuthType = watch("authType");
  const githubAppType = watch("githubAppAuth.appType");

  // Reset test result when auth type changes
  useEffect(() => {
    setTestResult(null);
  }, [selectedAuthType]);

  const handleTestConnection = async () => {
    if (!onTestConnection) return;

    const formData = watch();
    setIsTesting(true);
    setTestResult(null);

    try {
      const request: TestConnectionRequest = {
        repoURL: formData.repoURL,
        authType: formData.authType,
      };

      // Add auth-specific data
      if (formData.authType === "ssh" && formData.sshAuth) {
        request.sshAuth = {
          privateKey: formData.sshAuth.privateKey,
        };
      } else if (formData.authType === "https" && formData.httpsAuth) {
        request.httpsAuth = {
          username: formData.httpsAuth.username || undefined,
          password: formData.httpsAuth.password || undefined,
          bearerToken: formData.httpsAuth.bearerToken || undefined,
          tlsClientCert: formData.httpsAuth.tlsClientCert || undefined,
          tlsClientKey: formData.httpsAuth.tlsClientKey || undefined,
          insecureSkipTLSVerify: formData.httpsAuth.insecureSkipTLSVerify || undefined,
        };
      } else if (formData.authType === "github-app" && formData.githubAppAuth) {
        request.githubAppAuth = {
          appType: formData.githubAppAuth.appType,
          appId: formData.githubAppAuth.appId,
          installationId: formData.githubAppAuth.installationId,
          privateKey: formData.githubAppAuth.privateKey,
          enterpriseUrl: formData.githubAppAuth.enterpriseUrl || undefined,
        };
      }

      const result = await onTestConnection(request);
      setTestResult(result);
    } catch (error) {
      setTestResult({
        valid: false,
        message: error instanceof Error ? error.message : "Connection test failed",
      });
    } finally {
      setIsTesting(false);
    }
  };

  const onSubmitForm = async (data: RepositoryFormData) => {
    const request: CreateRepositoryRequest = {
      name: data.name,
      projectId: data.projectId,
      repoURL: data.repoURL,
      authType: data.authType,
      defaultBranch: data.defaultBranch,
    };

    // Add auth-specific data
    if (data.authType === "ssh" && data.sshAuth) {
      request.sshAuth = {
        privateKey: data.sshAuth.privateKey,
      };
    } else if (data.authType === "https" && data.httpsAuth) {
      request.httpsAuth = {
        username: data.httpsAuth.username || undefined,
        password: data.httpsAuth.password || undefined,
        bearerToken: data.httpsAuth.bearerToken || undefined,
        tlsClientCert: data.httpsAuth.tlsClientCert || undefined,
        tlsClientKey: data.httpsAuth.tlsClientKey || undefined,
        insecureSkipTLSVerify: data.httpsAuth.insecureSkipTLSVerify || undefined,
      };
    } else if (data.authType === "github-app" && data.githubAppAuth) {
      request.githubAppAuth = {
        appType: data.githubAppAuth.appType,
        appId: data.githubAppAuth.appId,
        installationId: data.githubAppAuth.installationId,
        privateKey: data.githubAppAuth.privateKey,
        enterpriseUrl: data.githubAppAuth.enterpriseUrl || undefined,
      };
    }

    await onSubmit(request);
  };

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
          {projects.map((project) => (
            <option key={project.name} value={project.name}>
              {project.name}
              {project.description && ` - ${project.description}`}
            </option>
          ))}
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
          placeholder={
            selectedAuthType === "ssh"
              ? "git@github.com:owner/repo.git"
              : "https://github.com/owner/repo.git"
          }
        />
        {errors.repoURL && <p className={errorClasses}>{errors.repoURL.message}</p>}
        <p className="mt-1 text-xs text-muted-foreground">
          {selectedAuthType === "ssh"
            ? "SSH format: git@github.com:owner/repo.git"
            : "HTTPS format: https://github.com/owner/repo.git"}
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
        <label className={labelClasses}>Authentication Method *</label>
        <div className="flex gap-2 mt-2">
          {(["ssh", "https", "github-app"] as AuthType[]).map((type) => (
            <button
              key={type}
              type="button"
              onClick={() => setValue("authType", type)}
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
        <div className="p-4 border border-border rounded-lg bg-muted/30 space-y-4">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Info className="h-4 w-4" />
            <span>SSH authentication using a private key</span>
          </div>

          <div>
            <div className="flex justify-between items-center mb-1.5">
              <label htmlFor="sshPrivateKey" className="text-sm font-medium">
                SSH Private Key *
              </label>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => setShowPrivateKey(!showPrivateKey)}
              >
                {showPrivateKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </Button>
            </div>
            <textarea
              id="sshPrivateKey"
              {...register("sshAuth.privateKey")}
              className={`${inputClasses} font-mono text-xs min-h-[120px]`}
              placeholder={showPrivateKey ? "-----BEGIN OPENSSH PRIVATE KEY-----\n...\n-----END OPENSSH PRIVATE KEY-----" : "••••••••••••"}
              style={showPrivateKey ? {} : { WebkitTextSecurity: "disc" } as React.CSSProperties}
            />
            {errors.sshAuth?.privateKey && (
              <p className={errorClasses}>{errors.sshAuth.privateKey.message}</p>
            )}
            <p className="mt-1 text-xs text-muted-foreground">
              Paste your SSH private key in PEM format
            </p>
          </div>
        </div>
      )}

      {/* HTTPS Authentication Fields */}
      {selectedAuthType === "https" && (
        <div className="p-4 border border-border rounded-lg bg-muted/30 space-y-4">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Info className="h-4 w-4" />
            <span>HTTPS authentication - provide at least one method</span>
          </div>

          {/* Username & Password */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label htmlFor="httpsUsername" className="text-sm font-medium">
                Username
              </label>
              <input
                id="httpsUsername"
                {...register("httpsAuth.username")}
                className={inputClasses}
                placeholder="git"
              />
            </div>
            <div>
              <div className="flex justify-between items-center">
                <label htmlFor="httpsPassword" className="text-sm font-medium">
                  Password / Token
                </label>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => setShowPassword(!showPassword)}
                >
                  {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </Button>
              </div>
              <input
                id="httpsPassword"
                type={showPassword ? "text" : "password"}
                {...register("httpsAuth.password")}
                className={inputClasses}
                placeholder="••••••••"
              />
            </div>
          </div>

          {/* Bearer Token */}
          <div>
            <div className="flex justify-between items-center">
              <label htmlFor="httpsBearerToken" className="text-sm font-medium">
                Bearer Token
              </label>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => setShowBearerToken(!showBearerToken)}
              >
                {showBearerToken ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </Button>
            </div>
            <input
              id="httpsBearerToken"
              type={showBearerToken ? "text" : "password"}
              {...register("httpsAuth.bearerToken")}
              className={inputClasses}
              placeholder="ghp_xxxxxxxxxxxxxxxxxxxx"
            />
            <p className="mt-1 text-xs text-muted-foreground">
              GitHub Personal Access Token or OAuth token
            </p>
          </div>

          {/* TLS Client Certificate (collapsible) */}
          <details className="group">
            <summary className="cursor-pointer text-sm font-medium text-muted-foreground hover:text-foreground">
              TLS Client Certificate (Advanced)
            </summary>
            <div className="mt-3 space-y-3">
              <div>
                <label htmlFor="httpsTlsCert" className="text-sm font-medium">
                  TLS Client Certificate
                </label>
                <textarea
                  id="httpsTlsCert"
                  {...register("httpsAuth.tlsClientCert")}
                  className={`${inputClasses} font-mono text-xs min-h-[80px]`}
                  placeholder="-----BEGIN CERTIFICATE-----"
                />
              </div>
              <div>
                <label htmlFor="httpsTlsKey" className="text-sm font-medium">
                  TLS Client Key
                </label>
                <textarea
                  id="httpsTlsKey"
                  {...register("httpsAuth.tlsClientKey")}
                  className={`${inputClasses} font-mono text-xs min-h-[80px]`}
                  placeholder="-----BEGIN PRIVATE KEY-----"
                />
              </div>
            </div>
          </details>

          {errors.httpsAuth && (
            <p className={errorClasses}>
              {typeof errors.httpsAuth === "object" && "message" in errors.httpsAuth
                ? (errors.httpsAuth as { message?: string }).message
                : "Authentication configuration error"}
            </p>
          )}
        </div>
      )}

      {/* GitHub App Authentication Fields */}
      {selectedAuthType === "github-app" && (
        <div className="p-4 border border-border rounded-lg bg-muted/30 space-y-4">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Info className="h-4 w-4" />
            <span>GitHub App authentication - recommended for organizations</span>
          </div>

          {/* App Type */}
          <div>
            <label className="text-sm font-medium">GitHub App Type *</label>
            <div className="flex gap-2 mt-2">
              {(["github", "github-enterprise"] as GitHubAppType[]).map((type) => (
                <button
                  key={type}
                  type="button"
                  onClick={() => setValue("githubAppAuth.appType", type)}
                  className={`flex-1 px-3 py-1.5 text-sm rounded-md border transition-colors ${
                    githubAppType === type
                      ? "bg-primary text-primary-foreground border-primary"
                      : "bg-background border-border hover:bg-accent hover:text-accent-foreground"
                  }`}
                >
                  {type === "github" ? "GitHub.com" : "GitHub Enterprise"}
                </button>
              ))}
            </div>
          </div>

          {/* Enterprise URL (conditional) */}
          {githubAppType === "github-enterprise" && (
            <div>
              <label htmlFor="githubEnterpriseUrl" className="text-sm font-medium">
                Enterprise URL *
              </label>
              <input
                id="githubEnterpriseUrl"
                {...register("githubAppAuth.enterpriseUrl")}
                className={inputClasses}
                placeholder="https://github.mycompany.com"
              />
              {errors.githubAppAuth?.enterpriseUrl && (
                <p className={errorClasses}>{errors.githubAppAuth.enterpriseUrl.message}</p>
              )}
            </div>
          )}

          {/* App ID & Installation ID */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label htmlFor="githubAppId" className="text-sm font-medium">
                App ID *
              </label>
              <input
                id="githubAppId"
                {...register("githubAppAuth.appId")}
                className={inputClasses}
                placeholder="123456"
              />
              {errors.githubAppAuth?.appId && (
                <p className={errorClasses}>{errors.githubAppAuth.appId.message}</p>
              )}
            </div>
            <div>
              <label htmlFor="githubInstallId" className="text-sm font-medium">
                Installation ID *
              </label>
              <input
                id="githubInstallId"
                {...register("githubAppAuth.installationId")}
                className={inputClasses}
                placeholder="12345678"
              />
              {errors.githubAppAuth?.installationId && (
                <p className={errorClasses}>{errors.githubAppAuth.installationId.message}</p>
              )}
            </div>
          </div>

          {/* Private Key */}
          <div>
            <div className="flex justify-between items-center mb-1.5">
              <label htmlFor="githubAppPrivateKey" className="text-sm font-medium">
                Private Key *
              </label>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => setShowPrivateKey(!showPrivateKey)}
              >
                {showPrivateKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </Button>
            </div>
            <textarea
              id="githubAppPrivateKey"
              {...register("githubAppAuth.privateKey")}
              className={`${inputClasses} font-mono text-xs min-h-[120px]`}
              placeholder={showPrivateKey ? "-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----" : "••••••••••••"}
              style={showPrivateKey ? {} : { WebkitTextSecurity: "disc" } as React.CSSProperties}
            />
            {errors.githubAppAuth?.privateKey && (
              <p className={errorClasses}>{errors.githubAppAuth.privateKey.message}</p>
            )}
            <p className="mt-1 text-xs text-muted-foreground">
              Download the private key when creating your GitHub App
            </p>
          </div>
        </div>
      )}

      {/* Test Connection Button */}
      {onTestConnection && (
        <div>
          <Button
            type="button"
            variant="outline"
            onClick={handleTestConnection}
            disabled={isTesting || !watch("repoURL")}
            className="w-full"
          >
            {isTesting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {isTesting ? "Testing Connection..." : "Test Connection"}
          </Button>

          {testResult && (
            <div
              className={`mt-3 p-3 rounded-md flex items-start gap-2 ${
                testResult.valid
                  ? "bg-status-success/10 text-status-success"
                  : "bg-status-error/10 text-status-error"
              }`}
            >
              {testResult.valid ? (
                <CheckCircle2 className="h-5 w-5 flex-shrink-0 mt-0.5" />
              ) : (
                <AlertCircle className="h-5 w-5 flex-shrink-0 mt-0.5" />
              )}
              <p className="text-sm">{testResult.message}</p>
            </div>
          )}
        </div>
      )}

      {/* Form Actions */}
      <div className="flex justify-end gap-3 pt-4 border-t border-border">
        <Button type="button" variant="outline" onClick={onCancel} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button type="submit" disabled={isSubmitting || isLoading}>
          {isSubmitting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
          {initialData ? "Update Repository" : "Add Repository"}
        </Button>
      </div>
    </form>
  );
}
