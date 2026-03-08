import { useState, useEffect, useRef, useCallback } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { localAdminLogin } from '@/api/auth';
import { ApiError } from '@/api/client';
import { useUserStore } from '@/stores/userStore';
import { logger } from '@/lib/logger';

const loginSchema = z.object({
  username: z.string().min(1, 'Username is required'),
  password: z.string().min(1, 'Password is required'),
});

type LoginFormData = z.infer<typeof loginSchema>;

interface LocalAdminFormProps {
  onSuccess?: () => void;
}

export function LocalAdminForm({ onSuccess }: LocalAdminFormProps) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [countdown, setCountdown] = useState(0);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const login = useUserStore((state) => state.login);

  const clearCountdown = useCallback(() => {
    if (timerRef.current) {
      clearInterval(timerRef.current);
      timerRef.current = null;
    }
    setCountdown(0);
  }, []);

  // Cleanup timer on unmount
  useEffect(() => {
    return () => {
      if (timerRef.current) {
        clearInterval(timerRef.current);
      }
    };
  }, []);

  const startCountdown = useCallback((seconds: number) => {
    clearCountdown();
    setCountdown(seconds);
    timerRef.current = setInterval(() => {
      setCountdown((prev) => {
        if (prev <= 1) {
          if (timerRef.current) {
            clearInterval(timerRef.current);
            timerRef.current = null;
          }
          setError(null);
          return 0;
        }
        return prev - 1;
      });
    }, 1000);
  }, [clearCountdown]);

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<LoginFormData>({
    resolver: zodResolver(loginSchema),
  });

  const onSubmit = async (data: LoginFormData) => {
    setLoading(true);
    setError(null);
    clearCountdown();

    try {
      const resp = await localAdminLogin(data);
      login(resp.user, resp.expiresAt);
      onSuccess?.();
    } catch (err: unknown) {
      logger.error('[LocalAdminForm] Login failed:', err);
      const apiErr = err as ApiError | undefined;
      if (apiErr && apiErr.code === 'RATE_LIMIT_EXCEEDED') {
        const retryAfter = Number(apiErr.details?.retry_after) || 60;
        setError(`Too many login attempts. Try again in ${retryAfter} seconds.`);
        startCountdown(retryAfter);
      } else if (apiErr && typeof apiErr.code === 'string' && typeof apiErr.message === 'string') {
        setError(apiErr.message);
      } else {
        setError('Invalid username or password');
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
      <div className="space-y-2">
        <label
          htmlFor="username"
          className="text-sm font-medium text-foreground"
        >
          Username
        </label>
        <Input
          id="username"
          type="text"
          placeholder="admin"
          disabled={loading}
          {...register('username')}
          aria-invalid={errors.username ? 'true' : 'false'}
        />
        {errors.username && (
          <p className="text-sm text-destructive" role="alert">
            {errors.username.message}
          </p>
        )}
      </div>

      <div className="space-y-2">
        <label
          htmlFor="password"
          className="text-sm font-medium text-foreground"
        >
          Password
        </label>
        <Input
          id="password"
          type="password"
          placeholder="••••••••"
          disabled={loading}
          {...register('password')}
          aria-invalid={errors.password ? 'true' : 'false'}
        />
        {errors.password && (
          <p className="text-sm text-destructive" role="alert">
            {errors.password.message}
          </p>
        )}
      </div>

      {error && (
        <div className="rounded-md bg-destructive/10 p-3 border border-destructive/20">
          <p className="text-sm text-destructive">
            {countdown > 0
              ? `Too many login attempts. Try again in ${countdown} seconds.`
              : error}
          </p>
        </div>
      )}

      <Button type="submit" disabled={loading || countdown > 0} className="w-full">
        {loading ? 'Signing in...' : countdown > 0 ? `Wait ${countdown}s` : 'Sign in'}
      </Button>
    </form>
  );
}
