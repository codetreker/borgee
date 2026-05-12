// CS-3.2 — InstallPromptButton (蓝图 client-shape.md §1.1 PWA install).
//
// DOM 字面锁 (cs-3-content-lock §2):
//   <button data-cs3-install-button data-install-state="installable">安装 Borgee 桌面应用</button>
//
// installed/unavailable 时 return null (不渲染, 不准 disabled style 替代).
//
// Negative constraint: prompt() must be click-triggered; mount-time auto-prompt is forbidden by Chrome.
import React from 'react';
import { useInstallPrompt } from '../lib/cs3-install-prompt';
import { INSTALL_BUTTON_LABEL } from '../lib/cs3-permission-labels';

export interface InstallPromptButtonProps {
  /** Optional callback after user accepts/dismisses (caller may toast). */
  onOutcome?: (outcome: 'accepted' | 'dismissed' | 'unavailable') => void;
}

export default function InstallPromptButton({ onOutcome }: InstallPromptButtonProps) {
  const { state, prompt } = useInstallPrompt();
  if (state !== 'installable') return null; // installed / unavailable → null
  const onClick = async () => {
    const outcome = await prompt();
    onOutcome?.(outcome);
  };
  return (
    <button
      type="button"
      className="cs3-install-button"
      data-cs3-install-button
      data-install-state={state}
      onClick={onClick}
    >
      {INSTALL_BUTTON_LABEL}
    </button>
  );
}
