import { useState, useMemo, lazy, Suspense } from "react";
import { Link } from "react-router-dom";
import {
  ArrowLeft,
  Package,
  Clock,
  Layers,
  FileCode,
  Box,
  Loader2,
  AlertCircle,
  ExternalLink,
  FolderKanban,
} from "lucide-react";
import { useRGD, useRGDSchema, useRGDResourceGraph } from "@/hooks/useRGDs";
import type { CatalogRGD, FormProperty } from "@/types/rgd";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";

// Lazy load ResourceGraphView to code-split @xyflow/react (~200KB)
// This ensures ReactFlow is only loaded when user views the Resources tab
const ResourceGraphView = lazy(() =>
  import("@/components/graph").then((m) => ({ default: m.ResourceGraphView }))
);

type TabId = "overview" | "schema" | "resources";

interface Tab {
  id: TabId;
  label: string;
  icon: React.ReactNode;
}

const TABS: Tab[] = [
  { id: "overview", label: "Overview", icon: <Layers className="h-4 w-4" /> },
  { id: "schema", label: "Schema", icon: <FileCode className="h-4 w-4" /> },
  { id: "resources", label: "Resources", icon: <Box className="h-4 w-4" /> },
];

interface RGDDetailViewProps {
  rgd: CatalogRGD;
  onBack: () => void;
  onDeploy?: () => void;
}

export function RGDDetailView({ rgd, onBack, onDeploy }: RGDDetailViewProps) {
  const [activeTab, setActiveTab] = useState<TabId>("overview");

  // Fetch full RGD details
  const { data: fullRGD } = useRGD(rgd.name, rgd.namespace);
  const displayRGD = fullRGD || rgd;

  const normalizedTags = useMemo(() => {
    const category = (displayRGD.category || "uncategorized").toLowerCase();
    const uniqueTags = [...new Set(displayRGD.tags?.map((t) => t.toLowerCase()) || [])];
    return uniqueTags.filter((tag) => tag !== category);
  }, [displayRGD.tags, displayRGD.category]);

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Back button */}
      <button
        onClick={onBack}
        className="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
      >
        <ArrowLeft className="h-4 w-4" />
        Back to catalog
      </button>

      {/* Header */}
      <div className="rounded-lg border border-border bg-card p-6">
        <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-4">
          <div className="flex items-start gap-4">
            <div className="h-12 w-12 rounded-lg bg-secondary flex items-center justify-center shrink-0">
              <Box className="h-6 w-6 text-muted-foreground" />
            </div>
            <div>
              <h1 className="text-2xl font-bold tracking-tight text-foreground">
                {displayRGD.title || displayRGD.name}
              </h1>
              {displayRGD.title && displayRGD.title !== displayRGD.name && (
                <p className="text-sm text-muted-foreground font-mono mt-0.5">{displayRGD.name}</p>
              )}
              <div className="flex flex-wrap items-center gap-2 mt-2">
                {displayRGD.labels?.["knodex.io/project"] && (
                  <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium bg-primary/10 text-primary border border-primary/20">
                    <FolderKanban className="h-3.5 w-3.5" />
                    {displayRGD.labels["knodex.io/project"]}
                  </span>
                )}
                {displayRGD.version && (
                  <span className="px-2.5 py-1 rounded-md text-xs font-mono font-medium text-muted-foreground bg-secondary border border-border">
                    {displayRGD.version}
                  </span>
                )}
              </div>
            </div>
          </div>

          {onDeploy && (
            <Button
              onClick={onDeploy}
              className="gap-2 shrink-0"
            >
              <ExternalLink className="h-4 w-4" />
              Deploy
            </Button>
          )}
        </div>

        {/* Description */}
        {displayRGD.description && (
          <p className="mt-4 text-sm text-muted-foreground border-t border-border pt-4">
            {displayRGD.description}
          </p>
        )}

        {/* Tags */}
        <div className="flex flex-wrap gap-2 mt-4">
          <span className="px-2.5 py-1 rounded-md text-xs font-medium bg-primary/10 text-primary">
            {(displayRGD.category || "uncategorized").toLowerCase()}
          </span>
          {normalizedTags.map((tag) => (
            <span
              key={tag}
              className="px-2.5 py-1 rounded-md text-xs text-muted-foreground bg-secondary"
            >
              {tag}
            </span>
          ))}
        </div>

        {/* Meta */}
        <div className="flex flex-wrap gap-6 mt-4 pt-4 border-t border-border text-xs text-muted-foreground">
          <Link
            to={`/instances?rgd=${encodeURIComponent(displayRGD.name)}`}
            className="flex items-center gap-1.5 hover:text-primary hover:underline transition-colors cursor-pointer"
          >
            <Package className="h-3.5 w-3.5" />
            {displayRGD.instances} instance{displayRGD.instances !== 1 ? "s" : ""}
          </Link>
          <span className="flex items-center gap-1.5">
            <Clock className="h-3.5 w-3.5" />
            Updated {formatDate(displayRGD.updatedAt)}
          </span>
        </div>
      </div>

      {/* Tabs */}
      <div className="border-b border-border">
        <nav className="flex gap-1 -mb-px">
          {TABS.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={cn(
                "flex items-center gap-2 px-4 py-2.5 text-sm font-medium border-b-2 transition-colors",
                activeTab === tab.id
                  ? "border-primary text-foreground"
                  : "border-transparent text-muted-foreground hover:text-foreground hover:border-border"
              )}
            >
              {tab.icon}
              {tab.label}
            </button>
          ))}
        </nav>
      </div>

      {/* Tab content */}
      <div className="min-h-[300px]">
        {activeTab === "overview" && (
          <OverviewTab rgd={displayRGD} />
        )}
        {activeTab === "schema" && (
          <SchemaTab rgd={displayRGD} />
        )}
        {activeTab === "resources" && (
          <ResourcesTab rgd={displayRGD} />
        )}
      </div>
    </div>
  );
}

function OverviewTab({ rgd }: { rgd: CatalogRGD }) {
  return (
    <div className="grid gap-6 md:grid-cols-2">
      <div className="rounded-lg border border-border bg-card p-4">
        <h3 className="text-sm font-medium text-foreground mb-3">Details</h3>
        <dl className="space-y-2 text-sm">
          <div className="flex justify-between">
            <dt className="text-muted-foreground">Name</dt>
            <dd className="text-foreground font-mono">{rgd.name}</dd>
          </div>
          {rgd.labels?.["knodex.io/project"] && (
            <div className="flex justify-between">
              <dt className="text-muted-foreground">Project</dt>
              <dd className="text-foreground font-mono">{rgd.labels["knodex.io/project"]}</dd>
            </div>
          )}
          <div className="flex justify-between">
            <dt className="text-muted-foreground">Category</dt>
            <dd className="text-foreground">{rgd.category || "Uncategorized"}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-muted-foreground">Version</dt>
            <dd className="text-foreground font-mono">{rgd.version || "N/A"}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-muted-foreground">API Version</dt>
            <dd className="text-foreground font-mono">{rgd.apiVersion || "N/A"}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-muted-foreground">Kind</dt>
            <dd className="text-foreground font-mono">{rgd.kind || "N/A"}</dd>
          </div>
        </dl>
      </div>

      <div className="rounded-lg border border-border bg-card p-4">
        <h3 className="text-sm font-medium text-foreground mb-3">Timestamps</h3>
        <dl className="space-y-2 text-sm">
          <div className="flex justify-between">
            <dt className="text-muted-foreground">Created</dt>
            <dd className="text-foreground">{formatDate(rgd.createdAt)}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-muted-foreground">Updated</dt>
            <dd className="text-foreground">{formatDate(rgd.updatedAt)}</dd>
          </div>
        </dl>
      </div>

      {Object.keys(rgd.labels || {}).length > 0 && (
        <div className="rounded-lg border border-border bg-card p-4 md:col-span-2">
          <h3 className="text-sm font-medium text-foreground mb-3">Labels</h3>
          <div className="flex flex-wrap gap-2">
            {Object.entries(rgd.labels).map(([key, value]) => (
              <span
                key={key}
                className="px-2 py-1 rounded text-xs font-mono bg-secondary text-muted-foreground"
              >
                {key}: {value}
              </span>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function SchemaTab({ rgd }: { rgd: CatalogRGD }) {
  const { data: schemaResponse, isLoading, error } = useRGDSchema(rgd.name, rgd.namespace);

  if (isLoading) {
    return (
      <div className="rounded-lg border border-border bg-card p-6">
        <div className="flex items-center gap-3 mb-4">
          <FileCode className="h-5 w-5 text-muted-foreground" />
          <h3 className="text-sm font-medium text-foreground">Input Schema</h3>
        </div>
        <div className="h-[200px] flex items-center justify-center">
          <div className="flex items-center gap-2 text-muted-foreground">
            <Loader2 className="h-5 w-5 animate-spin" />
            <span className="text-sm">Loading schema...</span>
          </div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-lg border border-border bg-card p-6">
        <div className="flex items-center gap-3 mb-4">
          <FileCode className="h-5 w-5 text-muted-foreground" />
          <h3 className="text-sm font-medium text-foreground">Input Schema</h3>
        </div>
        <div className="h-[200px] flex flex-col items-center justify-center gap-2">
          <AlertCircle className="h-6 w-6 text-destructive" />
          <p className="text-sm text-destructive">Failed to load schema</p>
          <p className="text-xs text-muted-foreground">
            {error instanceof Error ? error.message : "Unknown error"}
          </p>
        </div>
      </div>
    );
  }

  if (!schemaResponse?.crdFound || !schemaResponse.schema) {
    return (
      <div className="rounded-lg border border-border bg-card p-6">
        <div className="flex items-center gap-3 mb-4">
          <FileCode className="h-5 w-5 text-muted-foreground" />
          <h3 className="text-sm font-medium text-foreground">Input Schema</h3>
        </div>
        <div className="h-[200px] flex flex-col items-center justify-center gap-2">
          <FileCode className="h-8 w-8 text-muted-foreground/50" />
          <p className="text-sm text-muted-foreground">No CRD schema found</p>
          <p className="text-xs text-muted-foreground">
            {schemaResponse?.error || "The CRD for this RGD may not exist yet."}
          </p>
        </div>
      </div>
    );
  }

  const { schema } = schemaResponse;

  return (
    <div className="rounded-lg border border-border bg-card p-6">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <FileCode className="h-5 w-5 text-muted-foreground" />
          <h3 className="text-sm font-medium text-foreground">Input Schema</h3>
        </div>
        <span className="text-xs font-mono text-muted-foreground bg-secondary px-2 py-1 rounded">
          {schema.group}/{schema.version}
        </span>
      </div>

      {schema.description && (
        <p className="text-sm text-muted-foreground mb-4 pb-4 border-b border-border">
          {schema.description}
        </p>
      )}

      <div className="space-y-3">
        <h4 className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
          Properties ({Object.keys(schema.properties).length})
        </h4>
        <div className="space-y-2">
          {Object.entries(schema.properties).map(([name, prop]) => (
            <SchemaProperty
              key={name}
              name={name}
              property={prop}
              required={schema.required?.includes(name)}
            />
          ))}
        </div>
      </div>
    </div>
  );
}

function SchemaProperty({
  name,
  property,
  required,
  depth = 0,
}: {
  name: string;
  property: FormProperty;
  required?: boolean;
  depth?: number;
}) {
  const hasNestedProps = property.type === "object" && property.properties && Object.keys(property.properties).length > 0;

  return (
    <div
      className={cn(
        "rounded-md border border-border bg-secondary/30 p-3",
        depth > 0 && "ml-4"
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="flex items-center gap-2">
          <span className="font-mono text-sm text-foreground">{name}</span>
          {required && (
            <span className="text-[10px] font-medium text-destructive uppercase">required</span>
          )}
        </div>
        <span className="text-xs font-mono text-muted-foreground bg-secondary px-1.5 py-0.5 rounded">
          {property.type}
          {property.format && ` (${property.format})`}
        </span>
      </div>

      {property.description && (
        <p className="mt-1 text-xs text-muted-foreground">{property.description}</p>
      )}

      {property.enum && property.enum.length > 0 && (
        <div className="mt-2 flex flex-wrap gap-1">
          <span className="text-[10px] text-muted-foreground uppercase mr-1">enum:</span>
          {property.enum.map((val, i) => (
            <span key={i} className="text-xs font-mono text-muted-foreground bg-secondary px-1.5 py-0.5 rounded">
              {String(val)}
            </span>
          ))}
        </div>
      )}

      {property.default !== undefined && (
        <div className="mt-2 text-xs">
          <span className="text-muted-foreground">default: </span>
          <span className="font-mono text-foreground">{JSON.stringify(property.default)}</span>
        </div>
      )}

      {hasNestedProps && (
        <div className="mt-3 space-y-2">
          {Object.entries(property.properties!).map(([nestedName, nestedProp]) => (
            <SchemaProperty
              key={nestedName}
              name={nestedName}
              property={nestedProp}
              required={property.required?.includes(nestedName)}
              depth={depth + 1}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function ResourcesTab({ rgd }: { rgd: CatalogRGD }) {
  const { data: resourceGraph, isLoading, error } = useRGDResourceGraph(rgd.name, rgd.namespace);

  if (isLoading) {
    return (
      <div className="rounded-lg border border-border bg-card p-6">
        <div className="flex items-center gap-3 mb-4">
          <Box className="h-5 w-5 text-muted-foreground" />
          <h3 className="text-sm font-medium text-foreground">Resource Graph</h3>
        </div>
        <div className="h-[400px] flex items-center justify-center">
          <div className="flex items-center gap-2 text-muted-foreground">
            <Loader2 className="h-5 w-5 animate-spin" />
            <span className="text-sm">Loading resources...</span>
          </div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-lg border border-border bg-card p-6">
        <div className="flex items-center gap-3 mb-4">
          <Box className="h-5 w-5 text-muted-foreground" />
          <h3 className="text-sm font-medium text-foreground">Resource Graph</h3>
        </div>
        <div className="h-[200px] flex flex-col items-center justify-center gap-2">
          <AlertCircle className="h-6 w-6 text-destructive" />
          <p className="text-sm text-destructive">Failed to load resources</p>
          <p className="text-xs text-muted-foreground">
            {error instanceof Error ? error.message : "Unknown error"}
          </p>
        </div>
      </div>
    );
  }

  if (!resourceGraph) {
    return null;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Box className="h-5 w-5 text-muted-foreground" />
        <h3 className="text-sm font-medium text-foreground">Resource Graph</h3>
        <span className="text-xs text-muted-foreground">
          K8s resources defined in this RGD
        </span>
      </div>
      <Suspense
        fallback={
          <div className="h-[500px] rounded-lg border border-border bg-card flex items-center justify-center">
            <div className="flex items-center gap-2 text-muted-foreground">
              <Loader2 className="h-5 w-5 animate-spin" />
              <span className="text-sm">Loading graph visualization...</span>
            </div>
          </div>
        }
      >
        <ResourceGraphView resourceGraph={resourceGraph} />
      </Suspense>
    </div>
  );
}

function formatDate(dateString: string): string {
  if (!dateString) return "N/A";
  const date = new Date(dateString);
  return date.toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
