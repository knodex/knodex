import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { LocalAdminForm } from './LocalAdminForm';
import { ApiError } from '@/api/client';
import * as authApi from '@/api/auth';

// Mock the auth API
vi.mock('@/api/auth', () => ({
  localAdminLogin: vi.fn(),
}));

// Mock the user store
vi.mock('@/stores/userStore', () => ({
  useUserStore: vi.fn((selector) =>
    selector({
      login: vi.fn(),
      logout: vi.fn(),
      user: null,
      projects: [],
      roles: {},
      currentProject: null,
      // NOTE: isGlobalAdmin was removed - use useCanI() hook for permission checks
      isAuthenticated: false,
      token: null,
      setCurrentProject: vi.fn(),
      refreshUser: vi.fn(),
      isTokenExpired: vi.fn(() => false),
    })
  ),
}));

describe('LocalAdminForm', () => {
  const mockOnSuccess = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders username and password fields', () => {
    render(<LocalAdminForm onSuccess={mockOnSuccess} />);

    expect(screen.getByLabelText(/username/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
  });

  it('displays validation errors for empty fields', async () => {
    const user = userEvent.setup();
    render(<LocalAdminForm onSuccess={mockOnSuccess} />);

    const submitButton = screen.getByRole('button', { name: /sign in/i });
    await user.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText(/username is required/i)).toBeInTheDocument();
      expect(screen.getByText(/password is required/i)).toBeInTheDocument();
    });
  });

  it('submits form with valid credentials', async () => {
    const user = userEvent.setup();
    const mockToken = 'mock-jwt-token';
    vi.mocked(authApi.localAdminLogin).mockResolvedValue(mockToken);

    render(<LocalAdminForm onSuccess={mockOnSuccess} />);

    const usernameInput = screen.getByLabelText(/username/i);
    const passwordInput = screen.getByLabelText(/password/i);
    const submitButton = screen.getByRole('button', { name: /sign in/i });

    await user.type(usernameInput, 'admin');
    await user.type(passwordInput, 'password123');
    await user.click(submitButton);

    await waitFor(() => {
      expect(authApi.localAdminLogin).toHaveBeenCalledWith({
        username: 'admin',
        password: 'password123',
      });
      expect(mockOnSuccess).toHaveBeenCalled();
    });
  });

  it('displays error message on login failure', async () => {
    const user = userEvent.setup();
    vi.mocked(authApi.localAdminLogin).mockRejectedValue(
      new Error('Invalid credentials')
    );

    render(<LocalAdminForm onSuccess={mockOnSuccess} />);

    const usernameInput = screen.getByLabelText(/username/i);
    const passwordInput = screen.getByLabelText(/password/i);
    const submitButton = screen.getByRole('button', { name: /sign in/i });

    await user.type(usernameInput, 'admin');
    await user.type(passwordInput, 'wrongpassword');
    await user.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText(/invalid username or password/i)).toBeInTheDocument();
      expect(mockOnSuccess).not.toHaveBeenCalled();
    });
  });

  it('disables form during submission', async () => {
    const user = userEvent.setup();
    vi.mocked(authApi.localAdminLogin).mockImplementation(
      () => new Promise((resolve) => setTimeout(() => resolve('token'), 100))
    );

    render(<LocalAdminForm onSuccess={mockOnSuccess} />);

    const usernameInput = screen.getByLabelText(/username/i);
    const passwordInput = screen.getByLabelText(/password/i);
    const submitButton = screen.getByRole('button', { name: /sign in/i });

    await user.type(usernameInput, 'admin');
    await user.type(passwordInput, 'password123');
    await user.click(submitButton);

    // Form should be disabled during submission
    expect(usernameInput).toBeDisabled();
    expect(passwordInput).toBeDisabled();
    expect(submitButton).toBeDisabled();
    expect(submitButton).toHaveTextContent(/signing in/i);

    await waitFor(() => {
      expect(mockOnSuccess).toHaveBeenCalled();
    });
  });

  it('clears error message on new submission', async () => {
    const user = userEvent.setup();
    vi.mocked(authApi.localAdminLogin)
      .mockRejectedValueOnce(new Error('Invalid credentials'))
      .mockResolvedValueOnce('valid-token');

    render(<LocalAdminForm onSuccess={mockOnSuccess} />);

    const usernameInput = screen.getByLabelText(/username/i);
    const passwordInput = screen.getByLabelText(/password/i);
    const submitButton = screen.getByRole('button', { name: /sign in/i });

    // First attempt - fail
    await user.type(usernameInput, 'admin');
    await user.type(passwordInput, 'wrong');
    await user.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText(/invalid username or password/i)).toBeInTheDocument();
    });

    // Second attempt - success
    await user.clear(passwordInput);
    await user.type(passwordInput, 'correct');
    await user.click(submitButton);

    await waitFor(() => {
      expect(screen.queryByText(/invalid username or password/i)).not.toBeInTheDocument();
      expect(mockOnSuccess).toHaveBeenCalled();
    });
  });

  it('displays rate limit countdown on RATE_LIMIT_EXCEEDED error', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

    vi.mocked(authApi.localAdminLogin).mockRejectedValue(
      new ApiError('RATE_LIMIT_EXCEEDED', 'too many failed login attempts', 429, { retry_after: '5' })
    );

    render(<LocalAdminForm onSuccess={mockOnSuccess} />);

    const usernameInput = screen.getByLabelText(/username/i);
    const passwordInput = screen.getByLabelText(/password/i);
    const submitButton = screen.getByRole('button', { name: /sign in/i });

    await user.type(usernameInput, 'admin');
    await user.type(passwordInput, 'wrong');
    await user.click(submitButton);

    // Should show countdown message
    await waitFor(() => {
      expect(screen.getByText(/try again in 5 seconds/i)).toBeInTheDocument();
    });

    // Button should be disabled with countdown
    expect(screen.getByRole('button', { name: /wait 5s/i })).toBeDisabled();

    // Advance 1 second
    vi.advanceTimersByTime(1000);
    await waitFor(() => {
      expect(screen.getByText(/try again in 4 seconds/i)).toBeInTheDocument();
    });

    vi.useRealTimers();
  });

  it('shows server error message for non-rate-limit ApiError', async () => {
    const user = userEvent.setup();
    vi.mocked(authApi.localAdminLogin).mockRejectedValue(
      new ApiError('UNAUTHORIZED', 'invalid credentials', 401)
    );

    render(<LocalAdminForm onSuccess={mockOnSuccess} />);

    const usernameInput = screen.getByLabelText(/username/i);
    const passwordInput = screen.getByLabelText(/password/i);
    const submitButton = screen.getByRole('button', { name: /sign in/i });

    await user.type(usernameInput, 'admin');
    await user.type(passwordInput, 'wrong');
    await user.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('invalid credentials')).toBeInTheDocument();
    });
  });
});
