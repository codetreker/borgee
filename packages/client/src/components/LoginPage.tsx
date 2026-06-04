import React, { useState } from 'react';
import { login } from '../lib/api';

interface Props {
  onLogin: () => void | Promise<void>;
  onRegister?: () => void;
}

export default function LoginPage({ onLogin, onRegister }: Props) {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email || !password || loading) return;
    setLoading(true);
    setError('');
    try {
      await login(email, password);
      await onLogin();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="login-page">
      <div className="login-card">
        <h1 className="login-title">Borgee</h1>
        <form onSubmit={handleSubmit}>
          <label htmlFor="login-email" className="sr-only">Email</label>
          <input
            id="login-email"
            type="email"
            placeholder="Email"
            value={email}
            onChange={e => setEmail(e.target.value)}
            className="input-field login-input"
            autoFocus
          />
          <label htmlFor="login-password" className="sr-only">Password</label>
          <input
            id="login-password"
            type="password"
            placeholder="Password"
            value={password}
            onChange={e => setPassword(e.target.value)}
            className="input-field login-input"
          />
          {error && <div className="login-error">{error}</div>}
          <button
            type="submit"
            disabled={loading || !email || !password}
            className="btn btn-primary login-btn"
          >
            {loading ? 'Logging in...' : 'Log in'}
          </button>
        </form>
        {onRegister && (
          <div className="register-link">
            <a onClick={onRegister}>Have an invite code? Register</a>
          </div>
        )}
      </div>
    </div>
  );
}
