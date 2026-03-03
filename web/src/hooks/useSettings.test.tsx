import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useSettings } from "./useSettings";
import * as settingsApi from "@/api/settings";
import type { ReactNode } from "react";

vi.mock("@/api/settings");

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

describe("useSettings", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should fetch settings successfully", async () => {
    vi.mocked(settingsApi.getSettings).mockResolvedValue({
      organization: "acme",
    });

    const { result } = renderHook(() => useSettings(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual({ organization: "acme" });
    expect(settingsApi.getSettings).toHaveBeenCalledTimes(1);
  });

  it("should not retry on failure (AC #3 graceful degradation)", async () => {
    vi.mocked(settingsApi.getSettings).mockRejectedValue(
      new Error("Network error")
    );

    const { result } = renderHook(() => useSettings(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    // retry: false means getSettings is called exactly once, no retries
    expect(settingsApi.getSettings).toHaveBeenCalledTimes(1);
  });

  it("should not refetch on subsequent mounts (staleTime: Infinity)", async () => {
    vi.mocked(settingsApi.getSettings).mockResolvedValue({
      organization: "acme",
    });

    const wrapper = createWrapper();

    // First mount — fetches
    const { result, unmount } = renderHook(() => useSettings(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(settingsApi.getSettings).toHaveBeenCalledTimes(1);

    // Unmount and remount — staleTime: Infinity means no refetch
    unmount();
    const { result: result2 } = renderHook(() => useSettings(), { wrapper });

    await waitFor(() => {
      expect(result2.current.isSuccess).toBe(true);
    });

    // Still only called once — data served from cache
    expect(settingsApi.getSettings).toHaveBeenCalledTimes(1);
  });
});
