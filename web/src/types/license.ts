/** License information from the backend */
export interface LicenseInfo {
  licenseId: string;
  customer: string;
  edition: string;
  features: string[];
  maxUsers: number;
  issuedAt: string;
  expiresAt: string;
}

/** License status response from GET /api/v1/license */
export interface LicenseStatus {
  licensed: boolean;
  enterprise: boolean;
  status: "valid" | "grace_period" | "expired" | "missing" | "oss";
  message: string;
  license?: LicenseInfo;
  gracePeriodEnd?: string;
}
