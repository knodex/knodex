import { useLicenseStatus } from "@/hooks/useLicense";
import { isEnterprise } from "@/hooks/useCompliance";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

/**
 * LicenseStatusBanner displays a warning banner when the enterprise license
 * is missing, expired, or in grace period. Only renders in enterprise builds.
 */
export function LicenseStatusBanner() {
  const { data: status } = useLicenseStatus();

  // Don't render in OSS builds or when license is valid
  if (!isEnterprise() || !status || status.status === "valid" || status.status === "oss") {
    return null;
  }

  if (status.status === "grace_period") {
    const endDate = status.gracePeriodEnd
      ? new Date(status.gracePeriodEnd).toLocaleDateString()
      : "soon";

    return (
      <Alert variant="warning" showIcon>
        <AlertTitle>License Expired - Grace Period Active</AlertTitle>
        <AlertDescription>
          Your enterprise license has expired. All features remain available until {endDate}.
          Please renew your license to avoid service interruption.
        </AlertDescription>
      </Alert>
    );
  }

  if (status.status === "expired") {
    return (
      <Alert variant="destructive" showIcon>
        <AlertTitle>License Expired</AlertTitle>
        <AlertDescription>
          Your enterprise license has expired and the grace period has ended.
          Enterprise features are unavailable. Please contact your administrator to renew.
        </AlertDescription>
      </Alert>
    );
  }

  if (status.status === "missing") {
    return (
      <Alert variant="default" showIcon>
        <AlertTitle>No Enterprise License</AlertTitle>
        <AlertDescription>
          No enterprise license is installed. Enterprise features are unavailable.
        </AlertDescription>
      </Alert>
    );
  }

  return null;
}
