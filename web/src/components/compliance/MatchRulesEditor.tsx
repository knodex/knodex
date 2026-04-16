// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useFieldArray, Controller, useFormContext } from "react-hook-form";
import { Plus, Trash2 } from "@/lib/icons";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiGroupSelector } from "./ApiGroupSelector";
import { KindSelector } from "./KindSelector";
import type { ConstraintFormValues } from "./hooks/useConstraintFormValidation";

export function MatchRulesEditor() {
  const { register, control, watch, formState: { errors } } = useFormContext<ConstraintFormValues>();

  const { fields, append, remove } = useFieldArray({
    control,
    name: "matchKinds",
  });

  const matchKinds = watch("matchKinds");

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <Label>Resource Kinds to Match</Label>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => append({ apiGroups: [], kinds: [] })}
          data-testid="add-match-kind-btn"
        >
          <Plus className="h-3.5 w-3.5 mr-1" />
          Add Kind
        </Button>
      </div>

      {fields.length === 0 ? (
        <p className="text-sm text-muted-foreground text-center py-4 border rounded-lg">
          No match rules configured. The constraint will not match
          any resources.
        </p>
      ) : (
        <div className="space-y-3">
          {fields.map((field, index) => (
            <div
              key={field.id}
              className="flex gap-2 items-start p-3 border rounded-lg"
            >
              <div className="flex-1 space-y-3">
                <div className="space-y-2">
                  <Label
                    htmlFor={`matchKinds.${index}.apiGroups`}
                    className="text-xs"
                  >
                    API Groups
                  </Label>
                  <Controller
                    control={control}
                    name={`matchKinds.${index}.apiGroups`}
                    render={({ field: controllerField }) => (
                      <ApiGroupSelector
                        value={controllerField.value}
                        onChange={controllerField.onChange}
                        placeholder="Select API groups..."
                        data-testid={`api-groups-selector-${index}`}
                      />
                    )}
                  />
                  <p className="text-xs text-muted-foreground">
                    Select &quot;core&quot; for the core API group (Pods, Services, etc.)
                  </p>
                </div>
                <div className="space-y-2">
                  <Label
                    htmlFor={`matchKinds.${index}.kinds`}
                    className="text-xs"
                  >
                    Kinds
                  </Label>
                  <Controller
                    control={control}
                    name={`matchKinds.${index}.kinds`}
                    render={({ field: controllerField }) => (
                      <KindSelector
                        value={controllerField.value}
                        onChange={controllerField.onChange}
                        apiGroups={matchKinds?.[index]?.apiGroups ?? []}
                        placeholder="Select kinds..."
                        data-testid={`kinds-selector-${index}`}
                      />
                    )}
                  />
                  {errors.matchKinds?.[index]?.kinds && (
                    <p className="text-xs text-destructive">
                      {errors.matchKinds[index].kinds?.message}
                    </p>
                  )}
                </div>
              </div>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                onClick={() => remove(index)}
                className="text-muted-foreground hover:text-destructive mt-6"
                aria-label="Remove match kind"
                data-testid={`remove-match-kind-${index}`}
              >
                <Trash2 className="h-4 w-4" />
              </Button>
            </div>
          ))}
        </div>
      )}

      {/* Namespace Filter */}
      <div className="space-y-2 pt-4 border-t">
        <Label htmlFor="matchNamespaces">
          Namespace Filter (optional)
        </Label>
        <Input
          id="matchNamespaces"
          {...register("matchNamespaces")}
          placeholder="e.g., default, production (comma-separated, empty = all namespaces)"
          data-testid="namespaces-input"
        />
        <p className="text-xs text-muted-foreground">
          Leave empty to match resources in all namespaces
        </p>
      </div>
    </div>
  );
}
