import React from 'react';

import {
  AdminSettings,
  bindSocialAccount,
  PrizeTierSetting,
  RechargeActivityPayload,
  RechargeRewardTier,
  SocialBindingPayload,
  Stats,
  Sub2APISettings,
  InvitationSettings
} from './api';
import { DashboardSection, emptyStats } from './appConstants';

export interface RechargeRewardTierDraft {
  id?: number;
  thresholdAmount: string;
  rewardAmount: string;
  sort: string;
}

export function formatDateTime(value: string) {
  return value ? value.replace('T', ' ').slice(0, 19) : '-';
}

export function toRechargeTierDrafts(tiers: RechargeRewardTier[] = []): RechargeRewardTierDraft[] {
  const source = tiers.length ? tiers : [{ thresholdAmount: 100, rewardAmount: 10, sort: 0 }];
  return source.map((tier, index) => ({
    id: tier.id,
    thresholdAmount: Number(tier.thresholdAmount).toFixed(2),
    rewardAmount: Number(tier.rewardAmount).toFixed(2),
    sort: String(tier.sort ?? index)
  }));
}

export function parseRechargeActivityPayload(input: {
  name: string;
  description: string;
  enabled: boolean;
  startAt: string;
  endAt: string;
  tiers: RechargeRewardTierDraft[];
}): RechargeActivityPayload | string {
  const name = input.name.trim();
  if (!name) return '请填写活动名称';
  if (input.startAt && input.endAt && new Date(input.endAt).getTime() <= new Date(input.startAt).getTime()) {
    return '结束时间必须晚于开始时间';
  }
  if (input.tiers.length === 0) return '请至少配置一个奖励档位';
  const tiers = input.tiers.map((tier, index) => ({
    id: tier.id,
    thresholdAmount: Number(tier.thresholdAmount),
    rewardAmount: Number(tier.rewardAmount),
    sort: Number(tier.sort || index)
  }));
  if (tiers.some((tier) => !Number.isFinite(tier.thresholdAmount) || tier.thresholdAmount <= 0)) {
    return '达标充值金额必须大于 0';
  }
  if (tiers.some((tier) => !Number.isFinite(tier.rewardAmount) || tier.rewardAmount <= 0)) {
    return '奖励余额必须大于 0';
  }
  return {
    name,
    description: input.description.trim(),
    enabled: input.enabled,
    startAt: input.startAt,
    endAt: input.endAt,
    tiers
  };
}

export function toDateTimeLocal(value?: string | null) {
  if (!value) return '';
  return value.replace(' ', 'T').slice(0, 16);
}

export function getPendingSocialBindingFromURL(): SocialBindingPayload | null {
  const params = new URLSearchParams(window.location.search);
  const platform = params.get('platform')?.trim() ?? '';
  const userId = params.get('userid')?.trim() ?? '';
  if (!platform || !userId) {
    return null;
  }
  const inviteCode = params.get('invitecode')?.trim().toUpperCase() ?? '';
  return { platform, userId, ...(inviteCode ? { inviteCode } : {}) };
}

export function getInviteCodeFromURL() {
  return new URLSearchParams(window.location.search).get('invitecode')?.trim().toUpperCase() ?? '';
}

export async function bindPendingSocialAccount(binding: SocialBindingPayload | null) {
  if (!binding) {
    return;
  }
  try {
    const result = await bindSocialAccount(binding);
    if (result.invitation?.message) {
      const socialNotice = result.bound
        ? `已绑定 ${result.platform} 账号 ${result.externalUserId}；`
        : `当前账号已绑定 ${result.platform}；`;
      sessionStorage.setItem('social_binding_notice', `${socialNotice}${result.invitation.message}`);
    } else if (result.bound) {
      sessionStorage.setItem('social_binding_notice', `已绑定 ${result.platform} 账号 ${result.externalUserId}`);
    } else if (result.alreadyBound) {
      sessionStorage.setItem('social_binding_notice', `当前账号已绑定 ${result.platform}，本次不会覆盖`);
    }
  } catch (err) {
    const message = err instanceof Error ? err.message : '社交账号绑定失败';
    sessionStorage.setItem('social_binding_notice', `登录成功，但社交账号绑定失败：${message}`);
  }
}

export function consumeSocialBindingNotice() {
  const notice = sessionStorage.getItem('social_binding_notice') ?? '';
  if (notice) {
    sessionStorage.removeItem('social_binding_notice');
  }
  return notice;
}

export function formatOptionalDate(value: unknown) {
  return typeof value === 'string' && value ? formatDateTime(value) : '-';
}

export function formatPlatformName(value: string) {
  return value ? value.replace(/[_-]+/g, ' ') : '-';
}

export function formatToday() {
  return new Date().toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit'
  }).replaceAll('/', '-');
}

export function sectionTitle(section: DashboardSection) {
  switch (section) {
    case 'checkins':
      return '签到管理';
    case 'invitations':
      return '邀请记录';
    case 'favorites':
      return '网站收藏';
    case 'recharge':
      return '充值活动';
    case 'rates':
      return '倍率监控';
    case 'system':
      return '系统设置';
  }
}

export function isDashboardSection(value: unknown): value is DashboardSection {
  return ['checkins', 'invitations', 'favorites', 'recharge', 'rates', 'system'].includes(String(value));
}

export function normalizeStats(stats: Stats): Stats {
  return {
    ...emptyStats,
    ...stats,
    amountStats: Array.isArray(stats?.amountStats) ? stats.amountStats : []
  };
}

export function toPrizeTierDrafts(prizeTiers: PrizeTierSetting[] = []) {
  const tiers = prizeTiers.length ? prizeTiers : [{ amount: 1, probability: 100 }];
  return tiers.map((tier) => ({
    amount: Number(tier.amount).toFixed(2),
    probability: Number(tier.probability).toFixed(2)
  }));
}

export function toSub2APIDraft(settings: Sub2APISettings): Sub2APISettings {
  return {
    ...settings,
    authMode: settings.authMode || 'password',
    adminApiKey: '',
    adminPassword: '',
    timeoutSeconds: settings.timeoutSeconds || 15
  };
}

export function updateSub2APIDraft<K extends keyof Sub2APISettings>(
  key: K,
  value: Sub2APISettings[K],
  setSub2apiDraft: React.Dispatch<React.SetStateAction<Sub2APISettings>>,
  setSettingsSaved: React.Dispatch<React.SetStateAction<boolean>>
) {
  setSub2apiDraft((current) => ({ ...current, [key]: value }));
  setSettingsSaved(false);
}

export function parseSub2APIDraft(draft: Sub2APISettings): Sub2APISettings | string {
  const timeoutSeconds = Number(draft.timeoutSeconds);
  if (!Number.isInteger(timeoutSeconds) || timeoutSeconds <= 0) {
    return 'Sub2API 超时秒数必须是大于 0 的整数';
  }
  if (!['admin_api_key', 'password'].includes(draft.authMode)) {
    return '请选择有效的 Sub2API 认证方式';
  }
  return {
    ...draft,
    authMode: draft.authMode,
    baseUrl: draft.baseUrl.trim().replace(/\/+$/, ''),
    adminApiKey: draft.adminApiKey?.trim() ?? '',
    adminEmail: draft.adminEmail.trim(),
    timeoutSeconds
  };
}

export function toAmountOptions(amountStats: Stats['amountStats'], drafts: { amount: string; probability: string }[]) {
  const amounts = new Set<string>();
  amountStats.forEach((item) => {
    amounts.add(Number(item.amount).toFixed(2));
  });
  drafts.forEach((tier) => {
    const amount = Number(tier.amount);
    if (Number.isFinite(amount) && amount > 0) {
      amounts.add(amount.toFixed(2));
    }
  });
  return Array.from(amounts).sort((left, right) => Number(left) - Number(right));
}

export function updatePrizeTierDraft(
  index: number,
  key: 'amount' | 'probability',
  value: string,
  setPrizeTierDrafts: React.Dispatch<React.SetStateAction<{ amount: string; probability: string }[]>>,
  setSettingsSaved: React.Dispatch<React.SetStateAction<boolean>>
) {
  setPrizeTierDrafts((current) => current.map((tier, currentIndex) => (
    currentIndex === index ? { ...tier, [key]: value } : tier
  )));
  setSettingsSaved(false);
}

export function parsePrizeTierDrafts(drafts: { amount: string; probability: string }[]): PrizeTierSetting[] | string {
  if (drafts.length === 0) {
    return '请至少配置一个兑换码金额概率';
  }

  const tiers = drafts.map((tier) => ({
    amount: Number(tier.amount),
    probability: Number(tier.probability)
  }));

  if (tiers.some((tier) => !Number.isFinite(tier.amount) || tier.amount <= 0)) {
    return '金额必须大于 0';
  }
  if (tiers.some((tier) => !Number.isFinite(tier.probability) || tier.probability <= 0 || tier.probability > 100)) {
    return '概率必须大于 0 且不超过 100';
  }
  if (Math.abs(tiers.reduce((total, tier) => total + tier.probability, 0) - 100) > 0.001) {
    return '所有金额概率之和必须等于 100%';
  }

  return tiers;
}

export function prizeTierTotal(drafts: { amount: string; probability: string }[]) {
  return drafts.reduce((total, tier) => {
    const probability = Number(tier.probability);
    return Number.isFinite(probability) ? total + probability : total;
  }, 0);
}

export function settingsChanged(
  dailyMaxUsers: number,
  dailyMaxUsersDraft: string,
  dailyLimitMode: 'shared' | 'separate',
  dailyLimitModeDraft: 'shared' | 'separate',
  directDailyMaxUsers: number,
  directDailyMaxUsersDraft: string,
  socialDailyMaxUsers: number,
  socialDailyMaxUsersDraft: string,
  directPrizeTiers: PrizeTierSetting[],
  directPrizeTierDrafts: { amount: string; probability: string }[],
  socialPrizeTiers: PrizeTierSetting[],
  socialPrizeTierDrafts: { amount: string; probability: string }[],
  groupLink: string,
  groupLinkDraft: string,
  admin: AdminSettings,
  adminDraft: AdminSettings,
  sub2api: Sub2APISettings,
  sub2apiDraft: Sub2APISettings,
  invitation: InvitationSettings,
  invitationDraft: InvitationSettings
) {
  if (dailyMaxUsersDraft !== String(dailyMaxUsers)) {
    return true;
  }
  if (dailyLimitModeDraft !== dailyLimitMode) {
    return true;
  }
  if (directDailyMaxUsersDraft !== String(directDailyMaxUsers)) {
    return true;
  }
  if (socialDailyMaxUsersDraft !== String(socialDailyMaxUsers)) {
    return true;
  }
  if (JSON.stringify(toPrizeTierDrafts(directPrizeTiers)) !== JSON.stringify(directPrizeTierDrafts)) {
    return true;
  }
  if (JSON.stringify(toPrizeTierDrafts(socialPrizeTiers)) !== JSON.stringify(socialPrizeTierDrafts)) {
    return true;
  }
  if (groupLinkDraft !== groupLink) {
    return true;
  }
  if (adminDraft.username !== admin.username) {
    return true;
  }
  if ((adminDraft.password ?? '') !== '') {
    return true;
  }
  if (invitation.afterTime !== invitationDraft.afterTime || Number(invitation.amount) !== Number(invitationDraft.amount)) {
    return true;
  }
  return JSON.stringify(toSub2APIDraft(sub2api)) !== JSON.stringify(sub2apiDraft);
}
