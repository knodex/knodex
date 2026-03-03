import apiClient from "./client";

export interface Settings {
  organization: string;
}

export async function getSettings(): Promise<Settings> {
  const response = await apiClient.get<Settings>("/v1/settings");
  return response.data;
}
