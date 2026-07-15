import {
  Bookmark,
  BookOpen,
  Code2,
  Globe2,
  KeyRound,
  Mail,
  ShoppingCart,
  Star,
  UserRound,
  Wrench
} from 'lucide-react';

import {
  AdminSettings,
  CheckInStats,
  RedeemCodeStatus,
  Stats,
  Sub2APIGroupRateMonitor,
  Sub2APISettings,
  InvitationSettings,
  TelegramSettings
} from './api';

export type DashboardSection = 'checkins' | 'invitations' | 'favorites' | 'recharge' | 'rates' | 'system';
export type LoginMode = 'user' | 'admin';

export const emptyStats: Stats = { total: 0, available: 0, assigned: 0, used: 0, voided: 0, amountStats: [] };
export const emptyCheckInStats: CheckInStats = { todayAmount: 0, todayUsers: 0, daily: [] };

export const emptyAdminSettings: AdminSettings = {
  username: 'admin',
  password: '',
  passwordSet: false
};

export const emptySub2APISettings: Sub2APISettings = {
  baseUrl: '',
  authMode: 'password',
  adminApiKey: '',
  adminApiKeySet: false,
  adminEmail: '',
  adminPassword: '',
  adminPasswordSet: false,
  timeoutSeconds: 15
};

export const emptyGroupRateMonitor: Sub2APIGroupRateMonitor = {
  settings: {
    enabled: true,
    refreshIntervalSeconds: 300,
    monitoredGroupIds: [],
    publicGroupIds: []
  },
  groups: [],
  series: [],
  logs: []
};

export const emptyTelegramSettings: TelegramSettings = {
  enabled: false,
  botToken: '',
  botTokenSet: false,
  apiBaseUrl: 'https://api.telegram.org',
  pollIntervalSeconds: 2,
  botUsername: '',
  connected: false
};

export const emptyInvitationSettings: InvitationSettings = {
  afterTime: '',
  amount: 0
};

export const statusText: Record<RedeemCodeStatus, string> = {
  AVAILABLE: '未绑定',
  ASSIGNED: '已绑定',
  USED: '已使用',
  VOIDED: '已作废'
};

export const favoriteIconPresets = [
  { value: 'preset:bookmark', label: '书签', icon: Bookmark },
  { value: 'preset:globe', label: '网站', icon: Globe2 },
  { value: 'preset:book', label: '文档', icon: BookOpen },
  { value: 'preset:tool', label: '工具', icon: Wrench },
  { value: 'preset:star', label: '星标', icon: Star },
  { value: 'preset:code', label: '代码', icon: Code2 },
  { value: 'preset:mail', label: '邮箱', icon: Mail },
  { value: 'preset:key', label: '密钥', icon: KeyRound },
  { value: 'preset:user', label: '账号', icon: UserRound },
  { value: 'preset:shop', label: '商城', icon: ShoppingCart }
];

export const favoriteEmojiPresets = [
  { value: 'preset:rocket', label: '启动', emoji: '🚀' },
  { value: 'preset:fire', label: '热门', emoji: '🔥' },
  { value: 'preset:bulb', label: '灵感', emoji: '💡' },
  { value: 'preset:heart', label: '喜欢', emoji: '❤️' },
  { value: 'preset:money', label: '财务', emoji: '💰' },
  { value: 'preset:chart', label: '数据', emoji: '📈' },
  { value: 'preset:lock', label: '安全', emoji: '🔒' },
  { value: 'preset:gift', label: '福利', emoji: '🎁' }
];
