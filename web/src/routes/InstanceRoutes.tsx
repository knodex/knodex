import { useCallback } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { InstancesPage, InstanceDetailView } from "@/components/instances";
import { useInstance } from "@/hooks/useInstances";
import { useAnnouncements } from "@/hooks/useAnnouncements";
import type { Instance } from "@/types/rgd";
import { Loader2, AlertCircle } from "lucide-react";

export function InstancesRoute() {
  const navigate = useNavigate();

  const handleInstanceClick = useCallback((instance: Instance) => {
    // Use namespace/kind/name as the instance identifier
    navigate(`/instances/${encodeURIComponent(instance.namespace)}/${encodeURIComponent(instance.kind)}/${encodeURIComponent(instance.name)}`);
  }, [navigate]);

  return <InstancesPage onInstanceClick={handleInstanceClick} />;
}

export function InstanceDetailRoute() {
  const { namespace, kind, name } = useParams<{ namespace: string; kind: string; name: string }>();
  const navigate = useNavigate();
  const { announce } = useAnnouncements();

  // Fetch instance by namespace, kind, and name
  const { data: instance, isLoading, error } = useInstance(
    decodeURIComponent(namespace || ''),
    decodeURIComponent(kind || ''),
    decodeURIComponent(name || '')
  );

  const handleBack = useCallback(() => {
    navigate('/instances');
  }, [navigate]);

  const handleInstanceDeleted = useCallback(() => {
    announce("Instance deleted successfully", "polite");
    navigate('/instances');
  }, [navigate, announce]);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error || !instance) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[400px] gap-4">
        <AlertCircle className="h-12 w-12 text-destructive" />
        <div className="text-center">
          <h2 className="text-lg font-semibold">Instance Not Found</h2>
          <p className="text-sm text-muted-foreground">
            The instance "{namespace}/{kind}/{name}" could not be found.
          </p>
        </div>
        <button
          onClick={() => navigate('/instances')}
          className="px-4 py-2 rounded-md bg-primary text-primary-foreground hover:bg-primary/90"
        >
          Back to Instances
        </button>
      </div>
    );
  }

  return (
    <InstanceDetailView
      instance={instance}
      onBack={handleBack}
      onDeleted={handleInstanceDeleted}
    />
  );
}
