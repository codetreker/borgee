// cv-15-content-lock.test.ts — CV-15 byte-identical 跨 server/client/
// content-lock 同源验证 + 同义词grep 检查.

import { describe, it, expect } from 'vitest';
import {
  COMMENT_EDIT_HISTORY_ERR_TOAST,
} from '../lib/api';
import { COMMENT_EDIT_HISTORY_LABEL } from '../lib/comment_edit_history';

describe('CV-15 byte-identical 同源', () => {
  it('COMMENT_EDIT_HISTORY_LABEL 3 文案 byte-identical 跟 content-lock §1', () => {
    expect(COMMENT_EDIT_HISTORY_LABEL.title).toBe('编辑历史');
    expect(COMMENT_EDIT_HISTORY_LABEL.empty).toBe('暂无编辑记录');
    expect(COMMENT_EDIT_HISTORY_LABEL.count).toBe('共 N 次编辑');
  });

  it('COMMENT_EDIT_HISTORY_ERR_TOAST 3 错码 byte-identical', () => {
    expect(COMMENT_EDIT_HISTORY_ERR_TOAST['comment.not_artifact_comment']).toBe('该消息不是 artifact 评论');
    expect(COMMENT_EDIT_HISTORY_ERR_TOAST['comment.not_owner']).toBe('仅评论作者可查看历史');
    expect(COMMENT_EDIT_HISTORY_ERR_TOAST['comment.message_not_found']).toBe('消息不存在');
    expect(Object.keys(COMMENT_EDIT_HISTORY_ERR_TOAST).sort()).toEqual([
      'comment.message_not_found',
      'comment.not_artifact_comment',
      'comment.not_owner',
    ]);
  });

  it('文案跟 DM-7 EditHistoryModal §1 同源 (`编辑历史` 字面 byte-identical)', () => {
    // CV-15 reuses DM-7 文案锁 §1 — 改 DM-7 必同步改 CV-15 (cross-grep 守门).
    expect(COMMENT_EDIT_HISTORY_LABEL.title).toBe('编辑历史');
  });
});
