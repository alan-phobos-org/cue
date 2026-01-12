import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { TokenSettings } from './TokenSettings';
import { api, TokenInfo, CreateTokenResponse } from '../api/client';

// Mock the API client
vi.mock('../api/client', () => ({
  api: {
    listTokens: vi.fn(),
    createToken: vi.fn(),
    deleteToken: vi.fn(),
  },
  AuthRequiredError: class extends Error {
    constructor() {
      super('Authentication required');
      this.name = 'AuthRequiredError';
    }
  },
}));

const mockTokens: TokenInfo[] = [
  {
    id: 'tok_123',
    user_cn: 'testuser',
    name: 'my-automation',
    created_at: '2025-01-01T00:00:00Z',
    expires_at: '2025-02-01T00:00:00Z',
    last_used_at: '2025-01-05T00:00:00Z',
  },
];

describe('TokenSettings', () => {
  const mockOnClose = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    (api.listTokens as ReturnType<typeof vi.fn>).mockResolvedValue(mockTokens);
  });

  afterEach(() => {
    cleanup();
  });

  it('renders token settings modal', async () => {
    render(<TokenSettings onClose={mockOnClose} />);

    expect(screen.getByRole('heading', { name: 'API Tokens' })).toBeTruthy();
    expect(screen.getByPlaceholderText(/token name/i)).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Create Token' })).toBeTruthy();
  });

  it('loads and displays existing tokens', async () => {
    render(<TokenSettings onClose={mockOnClose} />);

    await waitFor(() => {
      expect(screen.getByText('my-automation')).toBeTruthy();
    });
  });

  it('creates a new token', async () => {
    const mockResponse: CreateTokenResponse = {
      id: 'tok_new',
      name: 'new-token',
      token: 'eyJ0ZXN0IjoidG9rZW4ifQ.signature',
      created_at: '2025-01-10T00:00:00Z',
      expires_at: '2025-02-10T00:00:00Z',
    };
    (api.createToken as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    render(<TokenSettings onClose={mockOnClose} />);

    const input = screen.getByPlaceholderText(/token name/i);
    fireEvent.change(input, { target: { value: 'new-token' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create Token' }));

    await waitFor(() => {
      expect(api.createToken).toHaveBeenCalledWith({ name: 'new-token' });
    });

    // Should show token modal with the actual token
    await waitFor(() => {
      expect(screen.getByText(/will not be shown again/i)).toBeTruthy();
    });
  });

  it('deletes a token after confirmation', async () => {
    (api.deleteToken as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
    vi.spyOn(window, 'confirm').mockReturnValue(true);

    render(<TokenSettings onClose={mockOnClose} />);

    await waitFor(() => {
      expect(screen.getByText('my-automation')).toBeTruthy();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Revoke' }));

    await waitFor(() => {
      expect(api.deleteToken).toHaveBeenCalledWith('tok_123');
    });
  });

  it('closes modal when clicking close button', async () => {
    render(<TokenSettings onClose={mockOnClose} />);

    fireEvent.click(screen.getByRole('button', { name: '×' }));

    expect(mockOnClose).toHaveBeenCalled();
  });

  it('shows empty state when no tokens exist', async () => {
    (api.listTokens as ReturnType<typeof vi.fn>).mockResolvedValue([]);

    render(<TokenSettings onClose={mockOnClose} />);

    await waitFor(() => {
      expect(screen.getByText(/no api tokens yet/i)).toBeTruthy();
    });
  });

  it('disables create button when name is empty', async () => {
    render(<TokenSettings onClose={mockOnClose} />);

    const button = screen.getByRole('button', { name: 'Create Token' });
    expect(button.getAttribute('disabled')).toBe('');
  });
});
