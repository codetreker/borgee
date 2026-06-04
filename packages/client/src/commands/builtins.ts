import { commandRegistry, CommandError } from './registry';
import type { CommandContext } from './registry';
import { displayChannelName } from '../lib/channelDisplay';

commandRegistry.register({
  name: 'help',
  description: '显示所有可用命令',
  usage: '/help',
  paramType: 'none',
  execute: async ({ channelId, dispatch }: CommandContext) => {
    const groups = commandRegistry.search('');
    const sections = groups.map(g => {
      const isSystem = g.group === 'System';
      const header = isSystem ? `── ${g.group} ──` : `── 🤖 ${g.group} ──`;
      const lines = g.items.map(c => `\`${c.usage}\` — ${c.description}`);
      return [header, ...lines].join('\n');
    });
    dispatch({
      type: 'INSERT_LOCAL_SYSTEM_MESSAGE',
      payload: { channelId, text: sections.join('\n\n') },
    });
  },
});

commandRegistry.register({
  name: 'leave',
  description: '离开当前频道',
  usage: '/leave',
  paramType: 'none',
  execute: async ({ channelId, api, dispatch }: CommandContext) => {
    const confirmed = window.confirm('确定离开当前频道？');
    if (!confirmed) return;
    await api.leaveChannel(channelId);
    dispatch({ type: 'NAVIGATE_AFTER_LEAVE', payload: { channelId } });
  },
});

commandRegistry.register({
  name: 'topic',
  description: '设置频道主题',
  usage: '/topic <text>',
  paramType: 'text',
  placeholder: '输入频道主题…',
  execute: async ({ channelId, args, api }: CommandContext) => {
    if (!args.trim()) throw new CommandError('Usage: /topic <text>');
    await api.updateChannel(channelId, { topic: args.trim() });
  },
});

commandRegistry.register({
  name: 'invite',
  description: '邀请用户加入频道',
  usage: '/invite @user',
  paramType: 'user',
  placeholder: '选择用户…',
  execute: async ({ channelId, resolvedUser, api }: CommandContext) => {
    if (!resolvedUser) throw new CommandError('Usage: /invite @user');
    await api.addChannelMember(channelId, resolvedUser.id);
  },
});

commandRegistry.register({
  name: 'dm',
  description: '打开与用户的私信',
  usage: '/dm @user',
  paramType: 'user',
  placeholder: '选择用户…',
  execute: async ({ resolvedUser, actions }: CommandContext) => {
    if (!resolvedUser) throw new CommandError('Usage: /dm @user');
    await actions.openDm(resolvedUser.id);
  },
});

commandRegistry.register({
  name: 'status',
  description: '显示频道状态',
  usage: '/status',
  paramType: 'none',
  execute: async ({ channelId, api, dispatch }: CommandContext) => {
    const { channel } = await api.getChannel(channelId);
    const members = await api.fetchChannelMembers(channelId);
    const onlineIds = await api.fetchOnlineUsers();
    const onlineSet = new Set(onlineIds);
    const online = members.filter(m => onlineSet.has(m.user_id));
    const offline = members.filter(m => !onlineSet.has(m.user_id));

    const lines = [
      `**#${displayChannelName(channel)}**`,
      `主题: ${channel.topic || '无'}`,
      `成员: ${members.length} (在线 ${online.length})`,
      '',
      online.length ? `🟢 在线: ${online.map(m => m.display_name).join(', ')}` : '',
      offline.length ? `⚫ 离线: ${offline.map(m => m.display_name).join(', ')}` : '',
    ].filter(Boolean);

    dispatch({
      type: 'INSERT_LOCAL_SYSTEM_MESSAGE',
      payload: { channelId, text: lines.join('\n') },
    });
  },
});

commandRegistry.register({
  name: 'clear',
  description: '清除本地聊天记录',
  usage: '/clear',
  paramType: 'none',
  execute: async ({ channelId, dispatch }: CommandContext) => {
    const confirmed = window.confirm('确定清除本地聊天记录？仅清除本地显示，不影响服务端数据。');
    if (!confirmed) return;
    dispatch({ type: 'CLEAR_LOCAL_MESSAGES', payload: { channelId } });
    dispatch({
      type: 'INSERT_LOCAL_SYSTEM_MESSAGE',
      payload: { channelId, text: '🗑️ 本地聊天记录已清除' },
    });
  },
});

commandRegistry.register({
  name: 'nick',
  description: '修改显示名',
  usage: '/nick <name>',
  paramType: 'text',
  placeholder: '新显示名…',
  execute: async ({ channelId, args, api, dispatch }: CommandContext) => {
    if (!args.trim()) throw new CommandError('Usage: /nick <name>');
    const oldUser = await api.fetchMe();
    const oldName = oldUser.display_name;
    await api.updateProfile({ display_name: args.trim() });
    dispatch({
      type: 'INSERT_LOCAL_SYSTEM_MESSAGE',
      payload: { channelId, text: `✅ 昵称已修改: ${oldName} → ${args.trim()}` },
    });
  },
});
