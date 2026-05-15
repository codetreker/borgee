import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import { ApiError, type Artifact, type ArtifactVersion } from '../lib/api';

const createdArtifact: Artifact = {
  id: 'art-1',
  channel_id: 'ch-1',
  type: 'markdown',
  title: 'Launch notes',
  body: 'Initial body',
  current_version: 1,
  created_at: 1700000000000,
  committer_kind: 'human',
  committer_id: 'u-1',
};

const createdVersion: ArtifactVersion = {
  version: 1,
  body: 'Initial body',
  committer_kind: 'human',
  committer_id: 'u-1',
  created_at: 1700000000000,
};

const apiMocks = vi.hoisted(() => ({
  commitArtifact: vi.fn(),
  createAnchor: vi.fn(),
  createArtifact: vi.fn(),
  getArtifact: vi.fn(),
  listAnchors: vi.fn(),
  listArtifactComments: vi.fn(),
  listArtifactVersions: vi.fn(),
  postArtifactComment: vi.fn(),
  rollbackArtifact: vi.fn(),
}));

const wsHookState = vi.hoisted(() => ({
  artifactUpdatedHandler: null as null | ((frame: { artifact_id: string; channel_id: string }) => void),
}));

vi.mock('../context/AppContext', () => ({
  useAppContext: () => ({
    state: {
      currentUser: {
        id: 'u-1',
        display_name: 'Owner',
        role: 'member',
        avatar_url: null,
        created_at: 1700000000000,
      },
      channels: [
        {
          id: 'ch-1',
          name: 'general',
          topic: '',
          created_at: 1700000000000,
          created_by: 'u-owner',
        },
      ],
    },
  }),
}));

vi.mock('../components/Toast', () => ({
  useToast: () => ({ showToast: vi.fn() }),
}));

vi.mock('../hooks/useWsHubFrames', () => ({
  useAnchorCommentAdded: vi.fn(),
  useArtifactCommentAdded: vi.fn(),
  useArtifactUpdated: vi.fn((handler: (frame: { artifact_id: string; channel_id: string }) => void) => {
    wsHookState.artifactUpdatedHandler = handler;
  }),
}));

vi.mock('../lib/api', () => ({
  ApiError: class ApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
  commitArtifact: apiMocks.commitArtifact,
  createAnchor: apiMocks.createAnchor,
  createArtifact: apiMocks.createArtifact,
  getArtifact: apiMocks.getArtifact,
  listAnchors: apiMocks.listAnchors,
  listArtifactComments: apiMocks.listArtifactComments,
  listArtifactVersions: apiMocks.listArtifactVersions,
  postArtifactComment: apiMocks.postArtifactComment,
  rollbackArtifact: apiMocks.rollbackArtifact,
}));

import ArtifactPanel from '../components/ArtifactPanel';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  apiMocks.createArtifact.mockResolvedValue(createdArtifact);
  apiMocks.listArtifactVersions.mockResolvedValue({ versions: [createdVersion] });
  apiMocks.listAnchors.mockResolvedValue({ anchors: [] });
  apiMocks.listArtifactComments.mockResolvedValue({ comments: [] });
  wsHookState.artifactUpdatedHandler = null;
});

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  root = null;
  if (container) {
    document.body.removeChild(container);
    container = null;
  }
  vi.clearAllMocks();
});

async function render(node: React.ReactElement) {
  root = createRoot(container!);
  await act(async () => {
    root!.render(node);
  });
}

describe('ArtifactPanel ArtifactComments production mount', () => {
  it('mounts ArtifactComments for the active artifact after create succeeds', async () => {
    await render(<ArtifactPanel channelId="ch-1" />);

    const openCreate = container!.querySelector('.artifact-empty .btn-primary') as HTMLButtonElement;
    await act(async () => {
      openCreate.click();
    });

    const submit = container!.querySelector('[data-testid="artifact-create-modal"] button[type="submit"]') as HTMLButtonElement;
    await act(async () => {
      submit.click();
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(container!.querySelector('[data-testid="cv5-artifact-comments"]')).toBeTruthy();
    expect(apiMocks.listArtifactComments).toHaveBeenCalledWith('art-1');
  });

  it('clears artifact content and shows a non-leaky forbidden state after artifact reload is denied', async () => {
    await render(<ArtifactPanel channelId="ch-1" />);

    const openCreate = container!.querySelector('.artifact-empty .btn-primary') as HTMLButtonElement;
    await act(async () => {
      openCreate.click();
    });

    const submit = container!.querySelector('[data-testid="artifact-create-modal"] button[type="submit"]') as HTMLButtonElement;
    await act(async () => {
      submit.click();
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(container!.textContent).toContain('Launch notes');
    expect(wsHookState.artifactUpdatedHandler).toBeTruthy();

    apiMocks.getArtifact.mockRejectedValue(
      new ApiError(403, 'private artifact Launch notes body secret'),
    );

    await act(async () => {
      wsHookState.artifactUpdatedHandler?.({ artifact_id: 'art-1', channel_id: 'ch-1' });
      await Promise.resolve();
      await Promise.resolve();
    });

    const forbidden = container!.querySelector('[data-artifact-forbidden]');
    expect(forbidden?.textContent).toBe('You do not have access to this artifact.');
    expect(container!.textContent).not.toContain('Launch notes');
    expect(container!.textContent).not.toContain('private artifact Launch notes body secret');
  });
});
