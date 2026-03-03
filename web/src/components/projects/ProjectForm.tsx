/**
 * Project form for creating and editing projects
 */
import { useState, useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Plus, X, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import type { Project, CreateProjectRequest, Destination } from "@/types/project";

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
  const nameValue = watch("name");

  // Auto-populate namespace with project name (only in create mode, when no destinations yet)
  useEffect(() => {
    if (!isEditing && destinations.length === 0 && !namespaceManuallyEdited && nameValue) {
      setNewDestNamespace(nameValue);
    }
  }, [nameValue, isEditing, destinations.length, namespaceManuallyEdited]);

  const handleFormSubmit = async (values: ProjectFormValues) => {
    await onSubmit({
      name: values.name,
      description: values.description,
      destinations: destinations.length > 0 ? destinations : undefined,
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
        <Label>Allowed Destinations</Label>
        <p className="text-xs text-muted-foreground mt-1 mb-3">
          Kubernetes namespaces where deployments are allowed. Use wildcards
          like dev-* for pattern matching.
        </p>

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
