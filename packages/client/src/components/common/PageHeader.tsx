// PageHeader — 通用页头, 左 ← 返回 / 右 × 关闭 / 中标题 / 右侧 actions.
//
// 旧多页 (SettingsPage / NodeManager 等) 都是手写 header
// + 各自 className, 只有 ← 没 ×. 用 useNavigation hook 做真正的入栈/出栈
// 后, 这个组件统一渲染. ← 调 back (栈空 fallback channel) / × 调 close
// (永远清栈回 channel).
//
// CSS 复用 .settings-page-header / .settings-back-btn / .settings-page-title
// 已有 token (避免新增重复样式), 加 .page-header-spacer / .page-header-close
// 给布局.
import React from 'react';
import { useNavigation } from '../Navigation/NavigationContext';

export interface PageHeaderProps {
  title: string;
  back?: boolean;
  close?: boolean;
  actions?: React.ReactNode;
}

export default function PageHeader({
  title,
  back = true,
  close = true,
  actions,
}: PageHeaderProps) {
  const nav = useNavigation();
  return (
    <header className="settings-page-header page-header">
      {back && (
        <button
          type="button"
          className="settings-back-btn page-header-back"
          onClick={nav.back}
          aria-label="返回"
        >
          ←
        </button>
      )}
      <h1 className="settings-page-title page-header-title">{title}</h1>
      <div className="page-header-spacer" />
      {actions}
      {close && (
        <button
          type="button"
          className="page-header-close"
          onClick={nav.close}
          aria-label="关闭"
        >
          ×
        </button>
      )}
    </header>
  );
}
