import React, { useState } from 'react';
import { register } from '../lib/api';

interface Props {
  onLogin: () => void | Promise<void>;
  onBack: () => void;
}

function validateEmail(email: string): string | null {
  if (!email) return null;
  const re = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  return re.test(email) ? null : 'Invalid email format';
}

function validatePassword(password: string): string | null {
  if (!password) return null;
  const byteLen = new TextEncoder().encode(password).length;
  if (byteLen < 8) return 'Password must be at least 8 characters';
  if (byteLen > 72) return 'Password must be at most 72 characters';
  return null;
}

function validateDisplayName(name: string): string | null {
  if (!name) return null;
  const trimmed = name.trim();
  if (trimmed.length < 1 || trimmed.length > 50) return 'Display name must be 1-50 characters';
  return null;
}

export default function RegisterPage({ onLogin, onBack }: Props) {
  const [inviteCode, setInviteCode] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [error, setError] = useState('');
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);

  const updateFieldError = (field: string, msg: string | null) => {
    setFieldErrors(prev => {
      if (msg) return { ...prev, [field]: msg };
      const next = { ...prev };
      delete next[field];
      return next;
    });
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (loading) return;

    const errors: Record<string, string> = {};
    const emailErr = validateEmail(email);
    if (emailErr) errors.email = emailErr;
    const pwErr = validatePassword(password);
    if (pwErr) errors.password = pwErr;
    const nameErr = validateDisplayName(displayName);
    if (nameErr) errors.displayName = nameErr;

    if (Object.keys(errors).length > 0) {
      setFieldErrors(errors);
      return;
    }

    setLoading(true);
    setError('');
    try {
      await register(inviteCode.trim(), email, password, displayName);
      await onLogin();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Registration failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="login-page">
      <div className="login-card">
        <h1 className="login-title">Borgee</h1>
        <h2 style={{ textAlign: 'center', fontSize: 16, marginBottom: 16, fontWeight: 400, color: 'var(--text-secondary)' }}>Create Account</h2>
        <form onSubmit={handleSubmit}>
          <label htmlFor="register-invite-code" className="sr-only">Invite Code</label>
          <input
            id="register-invite-code"
            type="text"
            placeholder="Invite Code"
            value={inviteCode}
            onChange={e => setInviteCode(e.target.value)}
            className="input-field login-input"
            autoFocus
          />
          <label htmlFor="register-display-name" className="sr-only">Display Name</label>
          <input
            id="register-display-name"
            type="text"
            placeholder="Display Name"
            value={displayName}
            onChange={e => {
              setDisplayName(e.target.value);
              updateFieldError('displayName', validateDisplayName(e.target.value));
            }}
            className="input-field login-input"
          />
          {fieldErrors.displayName && <div className="login-error" style={{ marginTop: -8, marginBottom: 8, fontSize: 13 }}>{fieldErrors.displayName}</div>}
          <label htmlFor="register-email" className="sr-only">Email</label>
          <input
            id="register-email"
            type="email"
            placeholder="Email"
            value={email}
            onChange={e => {
              setEmail(e.target.value);
              updateFieldError('email', validateEmail(e.target.value));
            }}
            className="input-field login-input"
          />
          {fieldErrors.email && <div className="login-error" style={{ marginTop: -8, marginBottom: 8, fontSize: 13 }}>{fieldErrors.email}</div>}
          <label htmlFor="register-password" className="sr-only">Password</label>
          <input
            id="register-password"
            type="password"
            placeholder="Password"
            value={password}
            onChange={e => {
              setPassword(e.target.value);
              updateFieldError('password', validatePassword(e.target.value));
            }}
            className="input-field login-input"
          />
          {fieldErrors.password && <div className="login-error" style={{ marginTop: -8, marginBottom: 8, fontSize: 13 }}>{fieldErrors.password}</div>}
          {error && <div className="login-error">{error}</div>}
          <button
            type="submit"
            disabled={loading || !inviteCode || !email || !password || !displayName}
            className="btn btn-primary login-btn"
          >
            {loading ? 'Creating account...' : 'Register'}
          </button>
        </form>
        <div className="register-link">
          <a onClick={onBack}>Already have an account? Log in</a>
        </div>
      </div>
    </div>
  );
}
