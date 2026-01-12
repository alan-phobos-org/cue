import { useState, useEffect } from 'react';
import { api, TokenInfo, CreateTokenResponse } from '../api/client';

interface TokenSettingsProps {
  onClose: () => void;
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });
}

function TokenCreatedModal({ token, onClose }: { token: string; onClose: () => void }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(token);
    setCopied(true);
    setTimeout(() => onClose(), 1500);
  };

  return (
    <div className="modal-overlay">
      <div className="modal token-modal">
        <h3>Token Created</h3>
        <p className="warning">
          Copy this token now. It will not be shown again.
        </p>
        <code className="token-display">{token}</code>
        <div className="modal-actions">
          <button className="btn btn-primary" onClick={handleCopy}>
            {copied ? 'Copied!' : 'Copy to Clipboard'}
          </button>
        </div>
      </div>
    </div>
  );
}

export function TokenSettings({ onClose }: TokenSettingsProps) {
  const [tokens, setTokens] = useState<TokenInfo[]>([]);
  const [newTokenName, setNewTokenName] = useState('');
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [createdToken, setCreatedToken] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadTokens();
  }, []);

  const loadTokens = async () => {
    try {
      const data = await api.listTokens();
      setTokens(data);
    } catch (err) {
      setError('Failed to load tokens');
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async () => {
    if (!newTokenName.trim()) return;

    setCreating(true);
    setError(null);
    try {
      const result: CreateTokenResponse = await api.createToken({ name: newTokenName });
      setCreatedToken(result.token);
      setNewTokenName('');
    } catch (err) {
      setError('Failed to create token');
      console.error(err);
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Revoke this token? This cannot be undone.')) return;

    try {
      await api.deleteToken(id);
      setTokens(prev => prev.filter((t) => t.id !== id));
    } catch (err) {
      setError('Failed to revoke token');
      console.error(err);
    }
  };

  const handleTokenModalClose = () => {
    setCreatedToken(null);
    loadTokens();
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal settings-modal" onClick={(e) => e.stopPropagation()}>
        <header className="modal-header">
          <h2>API Tokens</h2>
          <button className="close-btn" onClick={onClose}>
            &times;
          </button>
        </header>

        {error && <div className="error-message">{error}</div>}

        <div className="create-token-form">
          <input
            type="text"
            placeholder="Token name (e.g., 'my-script')"
            value={newTokenName}
            onChange={(e) => setNewTokenName(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
          />
          <button
            className="btn btn-primary"
            onClick={handleCreate}
            disabled={creating || !newTokenName.trim()}
          >
            {creating ? 'Creating...' : 'Create Token'}
          </button>
        </div>

        <div className="token-list">
          {loading ? (
            <div className="loading">Loading tokens...</div>
          ) : tokens.length === 0 ? (
            <div className="empty-tokens">
              No API tokens yet. Create one to access the API programmatically.
            </div>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Created</th>
                  <th>Expires</th>
                  <th>Last Used</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {tokens.map((token) => (
                  <tr key={token.id}>
                    <td>{token.name}</td>
                    <td>{formatDate(token.created_at)}</td>
                    <td>{formatDate(token.expires_at)}</td>
                    <td>{token.last_used_at ? formatDate(token.last_used_at) : 'Never'}</td>
                    <td>
                      <button
                        className="btn btn-danger btn-small"
                        onClick={() => handleDelete(token.id)}
                      >
                        Revoke
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        {createdToken && (
          <TokenCreatedModal token={createdToken} onClose={handleTokenModalClose} />
        )}
      </div>
    </div>
  );
}
