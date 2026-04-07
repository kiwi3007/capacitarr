import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ref, type Ref } from 'vue';

// Import after stubs are in place
import { useApi } from './useApi';

// ---------------------------------------------------------------------------
// Capture the ofetch.create factory call so we can inspect the interceptors.
// vi.hoisted ensures the variables are available in the vi.mock factory,
// which is hoisted to the top of the file before any other code runs.
// ---------------------------------------------------------------------------
const { capturedOptions, mockFetchInstance } = vi.hoisted(() => {
  const options: { value: Record<string, unknown> } = { value: {} };
  const instance = vi.fn();
  return { capturedOptions: options, mockFetchInstance: instance };
});

vi.mock('ofetch', () => ({
  ofetch: {
    create: (opts: Record<string, unknown>) => {
      capturedOptions.value = opts;
      return mockFetchInstance;
    },
  },
}));

// ---------------------------------------------------------------------------
// Mock Nuxt auto-imports
// ---------------------------------------------------------------------------

function mockUseRuntimeConfig() {
  return {
    public: {
      apiBaseUrl: 'http://localhost:2187',
    },
    app: {
      baseURL: '/',
    },
  };
}

const mockAuthCookie: Ref<string | null> = ref('true');
function mockUseAuthCookie() {
  return mockAuthCookie;
}

const mockOnConnectionLost = vi.fn();
const mockOnConnectionRestored = vi.fn();
function mockUseConnectionHealth() {
  return {
    onConnectionLost: mockOnConnectionLost,
    onConnectionRestored: mockOnConnectionRestored,
  };
}

const mockRouterPush = vi.fn();
function mockUseRouter() {
  return { push: mockRouterPush };
}

vi.stubGlobal('useRuntimeConfig', mockUseRuntimeConfig);
vi.stubGlobal('useAuthCookie', mockUseAuthCookie);
vi.stubGlobal('useConnectionHealth', mockUseConnectionHealth);
vi.stubGlobal('useRouter', mockUseRouter);

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('useApi', () => {
  beforeEach(() => {
    capturedOptions.value = {};
    mockFetchInstance.mockReset();
    mockOnConnectionLost.mockReset();
    mockOnConnectionRestored.mockReset();
    mockRouterPush.mockReset();
    mockAuthCookie.value = 'true';
  });

  // -------------------------------------------------------------------------
  // Return value
  // -------------------------------------------------------------------------
  describe('return value', () => {
    it('returns the ofetch instance created by ofetch.create', () => {
      const api = useApi();
      expect(api).toBe(mockFetchInstance);
    });
  });

  // -------------------------------------------------------------------------
  // ofetch.create configuration
  // -------------------------------------------------------------------------
  describe('ofetch.create configuration', () => {
    it('sets baseURL from runtime config', () => {
      useApi();
      expect(capturedOptions.value.baseURL).toBe('http://localhost:2187');
    });

    it('sets credentials to "include" for cookie-based auth', () => {
      useApi();
      expect(capturedOptions.value.credentials).toBe('include');
    });
  });

  // -------------------------------------------------------------------------
  // GET request formatting
  // -------------------------------------------------------------------------
  describe('GET request formatting', () => {
    it('delegates calls to the ofetch instance with URL and options', async () => {
      mockFetchInstance.mockResolvedValueOnce({ title: 'Firefly' });

      const api = useApi();
      const result = await api('/api/v1/shows/1');

      expect(mockFetchInstance).toHaveBeenCalledWith('/api/v1/shows/1');
      expect(result).toEqual({ title: 'Firefly' });
    });

    it('passes query parameters and options through to ofetch', async () => {
      mockFetchInstance.mockResolvedValueOnce([]);

      const api = useApi();
      await api('/api/v1/preview', { params: { force: 'true' } });

      expect(mockFetchInstance).toHaveBeenCalledWith('/api/v1/preview', {
        params: { force: 'true' },
      });
    });
  });

  // -------------------------------------------------------------------------
  // onResponse — connection restored
  // -------------------------------------------------------------------------
  describe('onResponse interceptor', () => {
    it('calls onConnectionRestored on any successful response', () => {
      useApi();
      const onResponse = capturedOptions.value.onResponse as () => void;
      expect(onResponse).toBeDefined();

      onResponse();
      expect(mockOnConnectionRestored).toHaveBeenCalledTimes(1);
    });
  });

  // -------------------------------------------------------------------------
  // onResponseError — 401 handling and connection restored
  // -------------------------------------------------------------------------
  describe('onResponseError interceptor', () => {
    it('clears auth cookie and redirects to /login on 401', () => {
      useApi();
      const onResponseError = capturedOptions.value.onResponseError as (ctx: {
        response: { status: number };
      }) => void;
      expect(onResponseError).toBeDefined();

      onResponseError({ response: { status: 401 } });

      expect(mockAuthCookie.value).toBeNull();
      expect(mockRouterPush).toHaveBeenCalledWith('/login');
    });

    it('does not redirect on non-401 errors', () => {
      useApi();
      const onResponseError = capturedOptions.value.onResponseError as (ctx: {
        response: { status: number };
      }) => void;

      onResponseError({ response: { status: 500 } });

      expect(mockAuthCookie.value).toBe('true');
      expect(mockRouterPush).not.toHaveBeenCalled();
    });

    it('calls onConnectionRestored even on error responses (backend is reachable)', () => {
      useApi();
      const onResponseError = capturedOptions.value.onResponseError as (ctx: {
        response: { status: number };
      }) => void;

      onResponseError({ response: { status: 500 } });
      expect(mockOnConnectionRestored).toHaveBeenCalledTimes(1);
    });
  });

  // -------------------------------------------------------------------------
  // onRequestError — connection lost
  // -------------------------------------------------------------------------
  describe('onRequestError interceptor', () => {
    it('calls onConnectionLost on network-level failures', () => {
      useApi();
      const onRequestError = capturedOptions.value.onRequestError as () => void;
      expect(onRequestError).toBeDefined();

      onRequestError();
      expect(mockOnConnectionLost).toHaveBeenCalledTimes(1);
    });
  });

  // -------------------------------------------------------------------------
  // Error handling
  // -------------------------------------------------------------------------
  describe('error handling', () => {
    it('propagates fetch errors to the caller', async () => {
      const networkError = new Error('Network error');
      mockFetchInstance.mockRejectedValueOnce(networkError);

      const api = useApi();
      await expect(api('/api/v1/shows')).rejects.toThrow('Network error');
    });

    it('propagates HTTP errors from ofetch', async () => {
      const httpError = Object.assign(new Error('Not Found'), {
        statusCode: 404,
        data: { error: 'Serenity not found' },
      });
      mockFetchInstance.mockRejectedValueOnce(httpError);

      const api = useApi();
      await expect(api('/api/v1/movies/999')).rejects.toThrow('Not Found');
    });
  });
});
