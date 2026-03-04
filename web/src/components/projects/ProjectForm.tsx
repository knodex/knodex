/**
 * Project form for creating and editing projects
 */
import { useState, useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Plus, X, Loader2, Shield, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent } from "@/components/ui/card";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { Project, CreateProjectRequest, Destination, ProjectRole } from "@/types/project";
import { PolicyRulesTable } from "./PolicyRulesTable";
import { OIDCGroupsManager } from "./OIDCGroupsManager";
import { ROLE_PRESETS, resolvePreset } from "@/lib/role-presets";

// DNS-1123 subdomain pattern for project names
const projectNameRegex = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/;

const projectFormSchema = z.object({
  name: z
    .string()
    .min(1, "Name is required")
    .max(63, "Name must be 63 characters or less")
    .regex(
      projectNameRegex,
      "Name must be lowercase alphanumeric, may contain hyphens, start/end with alphanumeric"
    ),
  description: z.string().max(500, "Description too long").optional(),
});

type ProjectFormValues = z.infer<typeof projectFormSchema>;

interface ProjectFormProps {
  initialData?: Project;
  onSubmit: (data: CreateProjectRequest) => Promise<void>;
  onCancel: () => void;
  isLoading?: boolean;
}

export function ProjectForm({
  initialData,
  onSubmit,
  onCancel,
  isLoading = false,
}: ProjectFormProps) {
  const isEditing = !!initialData;

  // Destinations state
  const [destinations, setDestinations] = useState<Destination[]>(
    initialData?.destinations || []
  );
  const [newDestNamespace, setNewDestNamespace] = useState("");
  // Track whether user has manually edited the namespace input
  const [namespaceManuallyEdited, setNamespaceManuallyEdited] = useState(false);
  // Roles state for project creation
  const [roles, setRoles] = useState<ProjectRole[]>([]);
  const [roleError, setRoleError] = useState<string | null>(null);
  const [destinationError, setDestinationError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    watch,
    formState: { errors },
  } = useForm<ProjectFormValues>({
    resolver: zodResolver(projectFormSchema),
    defaultValues: {
      name: initialData?.name || "",
      description: initialData?.description || "",
    },
  });

  // Watch the name field to auto-populate namespace default
  // eslint-disable-next-line react-hooks/incompatible-library -- watch() is needed for reactive form fields
  const nameValue = watch("name");

  // Auto-populate namespace with project name (only in create mode, when no destinations yet)
  useEffect(() => {
    if (!isEditing && destinations.length === 0 && !namespaceManuallyEdited && nameValue) {
      setNewDestNamespace(nameValue);
    }
  }, [nameValue, isEditing, destinations.length, namespaceManuallyEdited]);

  const handleFormSubmit = async (values: ProjectFormValues) => {
    // Validate destinations
    if (destinations.length === 0) {
      setDestinationError("At least one destination is required");
      return;
    }
    setDestinationError(null);
    // Validate roles before submit
    if (roles.length > 0) {
      const invalidRole = roles.find(r => !r.name?.trim());
      if (invalidRole) {
        setRoleError("All roles must have a name");
        return;
      }
      const noPolicyRole = roles.find(r => !r.policies?.length);
      if (noPolicyRole) {
        setRoleError(`Role "${noPolicyRole.name}" must have at least one policy`);
        return;
      }
    }
    setRoleError(null);
    await onSubmit({
      name: values.name,
      description: values.description,
      destinations: destinations.length > 0 ? destinations : undefined,
      roles: roles.length > 0 ? roles : undefined,
    });
  };

  const addDestination = () => {
    if (newDestNamespace.trim()) {
      setDestinations([
        ...destinations,
        {
          namespace: newDestNamespace.trim(),
        },
      ]);
      if (destinationError) setDestinationError(null);
      setNewDestNamespace("");
      setNamespaceManuallyEdited(false);
    }
  };

  const removeDestination = (index: number) => {
    const updated = destinations.filter((_, i) => i !== index);
    setDestinations(updated);
    // Reset auto-populate tracking when all destinations are removed
    if (updated.length === 0) {
      setNamespaceManuallyEdited(false);
    }
  };

  return (
    <form onSubmit={handleSubmit(handleFormSubmit)} className="space-y-6">
      {/* Basic Info */}
      <div className="space-y-4">
        <div>
          <Label htmlFor="name">
            Project Name <span className="text-destructive">*</span>
          </Label>
          <Input
            id="name"
            {...register("name")}
            placeholder="my-project"
            disabled={isEditing || isLoading}
            className="mt-1.5"
          />
          {errors.name && (
            <p className="mt-1 text-sm text-destructive">{errors.name.message}</p>
          )}
          <p className="mt-1 text-xs text-muted-foreground">
            DNS-compatible name (lowercase, alphanumeric, hyphens allowed)
          </p>
        </div>

        <div>
          <Label htmlFor="description">Description</Label>
          <Textarea
            id="description"
            {...register("description")}
            placeholder="A brief description of this project's purpose..."
            disabled={isLoading}
            className="mt-1.5"
            rows={3}
          />
          {errors.description && (
            <p className="mt-1 text-sm text-destructive">
              {errors.description.message}
            </p>
          )}
        </div>
      </div>

      {/* Destinations */}
      <div>
        <Label>
          Allowed Destinations <span className="text-destructive">*</span>
        </Label>
        <p className="text-xs text-muted-foreground mt-1 mb-3">
          Kubernetes namespaces where deployments are allowed. Use wildcards
          like dev-* for pattern matching.
        </p>
        {destinationError && (
          <p className="text-sm text-destructive mb-3">{destinationError}</p>
        )}

        {/* Existing destinations */}
        <div className="space-y-2 mb-3">
          {destinations.map((dest, index) => (
            <div
              key={index}
              className="flex items-center gap-2 p-2 bg-secondary rounded-md"
            >
              <span className="flex-1 text-xs">
                <span className="text-muted-foreground">Namespace:</span>{" "}
                <code>{dest.namespace || "*"}</code>
                {dest.name && (
                  <>
                    {" "}
                    <span className="text-muted-foreground">Name:</span>{" "}
                    <code>{dest.name}</code>
                  </>
                )}
              </span>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => removeDestination(index)}
                disabled={isLoading}
              >
                <X className="h-4 w-4" />
              </Button>
            </div>
          ))}
        </div>

        {/* Add new destination */}
        <div className="flex gap-2">
          <Input
            value={newDestNamespace}
            onChange={(e) => {
              setNewDestNamespace(e.target.value);
              setNamespaceManuallyEdited(true);
            }}
            placeholder="Namespace (e.g., my-project, dev-*)"
            disabled={isLoading}
            className="flex-1"
          />
          <Button
            type="button"
            variant="outline"
            onClick={addDestination}
            disabled={isLoading}
          >
            <Plus className="h-4 w-4" />
          </Button>
        </div>
      </div>

      {/* Roles (Optional) - only show in create mode */}
      {!isEditing && (
        <div>
          <Label>Roles (Optional)</Label>
          <p className="text-xs text-muted-foreground mt-1 mb-3">
            Add roles with policies and group mappings. You can also add roles later from the project detail page.
          </p>

          {/* Preset buttons */}
          <div className="flex flex-wrap gap-2 mb-3">
            {ROLE_PRESETS.map((preset) => {
              const exists = roles.some(r => r.name === preset.name);
              const disabled = isLoading || !nameValue || exists;
              return (
                <Tooltip key={preset.name}>
                  <TooltipTrigger asChild>
                    <span>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        disabled={disabled}
                        onClick={() => {
                          setRoles(prev => [...prev, resolvePreset(preset, nameValue)]);
                        }}
                      >
                        <Sparkles className="h-4 w-4 mr-1" />
                        {preset.label}
                      </Button>
                    </span>
                  </TooltipTrigger>
                  {exists && <TooltipContent>Role already added</TooltipContent>}
                  {!nameValue && !exists && <TooltipContent>Enter a project name first</TooltipContent>}
                </Tooltip>
              );
            })}
            <Tooltip>
              <TooltipTrigger asChild>
                <span>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    disabled={isLoading || !nameValue}
                    onClick={() => {
                      setRoles(prev => [...prev, { name: "", description: "", policies: [], groups: [] }]);
                    }}
                  >
                    <Plus className="h-4 w-4 mr-1" />
                    Custom Role
                  </Button>
                </span>
              </TooltipTrigger>
              {!nameValue && <TooltipContent>Enter a project name first</TooltipContent>}
            </Tooltip>
          </div>

          {/* Role cards */}
          {roles.map((role, index) => (
            <Card key={index} className="mb-3">
              <CardContent className="pt-4 space-y-4">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Shield className="h-4 w-4 text-primary" />
                    <span className="font-medium">{role.name || "Custom Role"}</span>
                    {role.description && (
                      <span className="text-xs text-muted-foreground">— {role.description}</span>
                    )}
                  </div>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() => setRoles(prev => prev.filter((_, i) => i !== index))}
                    disabled={isLoading}
                  >
                    <X className="h-4 w-4" />
                  </Button>
                </div>

                {/* Inline name/description for custom roles */}
                {!ROLE_PRESETS.some(p => p.name === role.name) && (
                  <div>
                    <Input
                      value={role.name}
                      onChange={(e) => {
                        const newName = e.target.value.toLowerCase().replace(/\s+/g, "-");
                        setRoles(prev => prev.map((r, i) => i === index ? { ...r, name: newName } : r));
                      }}
                      placeholder="Role name (e.g., deployer)"
                      disabled={isLoading}
                    />
                  </div>
                )}

                {/* PolicyRulesTable */}
                <div>
                  <Label className="text-muted-foreground mb-2 block">Policy Rules</Label>
                  <PolicyRulesTable
                    key={`${index}-${role.name}`}
                    projectId={nameValue}
                    roleName={role.name || "custom-role"}
                    policies={role.policies || []}
                    onPoliciesChange={(policies) => {
                      setRoles(prev => prev.map((r, i) => i === index ? { ...r, policies } : r));
                    }}
                    canEdit={true}
                    isLoading={isLoading}
                  />
                </div>

                {/* OIDCGroupsManager */}
                <OIDCGroupsManager
                  groups={role.groups || []}
                  onGroupsChange={(groups) => {
                    setRoles(prev => prev.map((r, i) => i === index ? { ...r, groups } : r));
                  }}
                  canEdit={true}
                  isLoading={isLoading}
                />
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Role validation error */}
      {roleError && (
        <p className="text-sm text-destructive">{roleError}</p>
      )}

      {/* Form Actions */}
      <div className="flex justify-end gap-3 pt-4 border-t">
        <Button
          type="button"
          variant="outline"
          onClick={onCancel}
          disabled={isLoading}
        >
          Cancel
        </Button>
        <Button type="submit" disabled={isLoading}>
          {isLoading && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
          {isEditing ? "Save Changes" : "Create Project"}
        </Button>
      </div>
    </form>
  );
}
