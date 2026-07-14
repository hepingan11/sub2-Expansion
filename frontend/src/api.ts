const API_BASE = import.meta.env.VITE_API_BASE ?? '';
const TOKEN_KEY = 'redeem_admin_token';
const USER_TOKEN_KEY = 'redeem_user_token';
const USER_REFRESH_TOKEN_KEY = 'redeem_user_refresh_token';

function redirectToLogin() {
  if (window.location.pathname !== '/login') {
    window.location.replace('/login');
  }
}

export type RedeemCodeStatus = 'AVAILABLE' | 'ASSIGNED' | 'USED' | 'VOIDED';

export interface RedeemCode {
  id: number;
  code: string;
  userId: string | null;
  signDate: string | null;
  amount: number;
  status: RedeemCodeStatus;
  createdAt: string;
  updatedAt: string;
}

export interface PageResult<T> {
  content: T[];
  totalElements: number;
  totalPages: number;
  number: number;
  size: number;
}

export interface Stats {
  total: number;
  available: number;
  assigned: number;
  used: number;
  voided: number;
  amountStats: AmountStats[];
}

export interface AmountStats {
  amount: number;
  total: number;
  available: number;
}

export interface CodePayload {
  code?: string;
  userId?: string;
  signDate?: string;
  amount: string;
  status: RedeemCodeStatus;
}

export interface BatchImportPayload {
  codesText: string;
  amount: string;
}

export interface BatchImportResult {
  totalParsed: number;
  imported: number;
  duplicated: number;
  duplicatedCodes: string[];
}

export interface FavoriteSite {
  id: number;
  icon: string;
  url: string;
  name: string;
  description: string;
  sort: number;
  group: string;
  createdAt: string;
  updatedAt: string;
}

export interface FavoriteSitePayload {
  icon: string;
  url: string;
  name: string;
  description: string;
  sort: number;
  group: string;
}

export interface CheckInSettings {
  dailyMaxUsers: number;
  dailyLimitMode: 'shared' | 'separate';
  directDailyMaxUsers: number;
  socialDailyMaxUsers: number;
  prizeTiers: PrizeTierSetting[];
  directPrizeTiers: PrizeTierSetting[];
  socialPrizeTiers: PrizeTierSetting[];
  groupLink: string;
  admin: AdminSettings;
  sub2api: Sub2APISettings;
  invitation: InvitationSettings;
}

export interface InvitationSettings {
  afterTime: string;
  amount: number;
}

export interface CheckInResult {
  success: boolean;
  alreadyCheckedIn: boolean;
  userId: string | null;
  signDate: string | null;
  code: string;
  amount: number;
  checkInMethod?: string;
  platformType?: string;
  groupLink?: string;
  socialPrizeTiers?: PrizeTierSetting[];
  message: string;
}

export interface DailyCheckInStat {
  signDate: string;
  amount: number;
  users: number;
}

export interface CheckInStats {
  todayAmount: number;
  todayUsers: number;
  daily: DailyCheckInStat[];
}

export interface PrizeTierSetting {
  amount: number;
  probability: number;
}

export interface Sub2APISettings {
  baseUrl: string;
  authMode: 'admin_api_key' | 'password';
  adminApiKey?: string;
  adminApiKeySet: boolean;
  adminEmail: string;
  adminPassword?: string;
  adminPasswordSet: boolean;
  timeoutSeconds: number;
}

export interface AdminSettings {
  username: string;
  password?: string;
  passwordSet: boolean;
}

export interface UserSocialBinding {
  id: number;
  userId: number;
  platform: string;
  externalUserId: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface Sub2APIUserProfile {
  id?: number;
  email?: string;
  username?: string;
  role?: string;
  balance?: number;
  concurrency?: number;
  status?: string;
  allowed_groups?: number[];
  total_recharged?: number;
  created_at?: string;
  updated_at?: string;
  run_mode?: string;
  socialBindings?: UserSocialBinding[];
  [key: string]: unknown;
}

export interface UserLoginResponse {
  access_token?: string;
  refresh_token?: string;
  expires_in?: number;
  token_type?: string;
  user?: Sub2APIUserProfile;
  requires_2fa?: boolean;
  temp_token?: string;
  user_email_masked?: string;
}

export interface RechargeRewardTier {
  id?: number;
  activityId?: number;
  thresholdAmount: number;
  rewardAmount: number;
  sort: number;
  createdAt?: string;
  updatedAt?: string;
}

export interface RechargeActivity {
  id: number;
  name: string;
  description: string;
  enabled: boolean;
  startAt: string | null;
  endAt: string | null;
  createdAt: string;
  updatedAt: string;
  tiers: RechargeRewardTier[];
}

export interface RechargeActivityPayload {
  name: string;
  description: string;
  enabled: boolean;
  startAt: string;
  endAt: string;
  tiers: RechargeRewardTier[];
}

export interface UserRechargeRewardTier {
  id: number;
  thresholdAmount: number;
  rewardAmount: number;
  eligible: boolean;
  claimed: boolean;
  claimStatus: string;
  redeemCode?: string;
  claimedAt?: string;
}

export interface UserRechargeActivity {
  id: number;
  name: string;
  description: string;
  startAt: string | null;
  endAt: string | null;
  tiers: UserRechargeRewardTier[];
}

export interface UserRechargeRewards {
  totalRecharged: number;
  activities: UserRechargeActivity[];
}

export interface ClaimRechargeRewardResult {
  claimId: number;
  redeemCode: string;
  rewardAmount: number;
}

export interface AdminRechargeRewardClaim {
  id: number;
  activityId: number;
  activityName: string;
  tierId: number;
  tierSort: number;
  userId: number;
  thresholdAmount: number;
  rewardAmount: number;
  status: string;
  redeemCode: string;
  errorMessage: string;
  createdAt: string;
  updatedAt: string;
}

export interface SocialBindingPayload {
  platform: string;
  userId: string;
  inviteCode?: string;
}

export interface SocialBindingResult {
  id: number;
  platform: string;
  externalUserId: string;
  bound: boolean;
  alreadyBound: boolean;
  message: string;
  invitation?: InvitationBindingResult;
}

export interface Sub2APIGroupRateMonitorSettings {
  enabled: boolean;
  refreshIntervalSeconds: number;
  monitoredGroupIds: string[];
  publicGroupIds: string[];
}

export interface Sub2APIGroupRateGroup {
  groupId: string;
  groupName: string;
  rateMultiplier: number;
  monitored: boolean;
  publicVisible: boolean;
  lastSeenAt: string;
}

export interface Sub2APIGroupRatePoint {
  time: string;
  rate: number;
}

export interface Sub2APIGroupRateSeries {
  groupId: string;
  groupName: string;
  publicVisible: boolean;
  points: Sub2APIGroupRatePoint[];
}

export interface Sub2APIGroupRateLog {
  id: number;
  groupId: string;
  groupName: string;
  oldRate: number;
  newRate: number;
  source: string;
  publicVisible: boolean;
  createdAt: string;
}

export interface Sub2APIGroupRateMonitor {
  settings: Sub2APIGroupRateMonitorSettings;
  groups: Sub2APIGroupRateGroup[];
  series: Sub2APIGroupRateSeries[];
  logs: Sub2APIGroupRateLog[];
}

export interface SystemUpdateCheck {
  currentVersion: string;
  latestVersion: string;
  updateAvailable: boolean;
  releaseUrl: string;
  releaseName: string;
  publishedAt: string;
  repository: string;
  updateEnabled: boolean;
  updateCommand?: string;
  message: string;
}

export interface InvitationBindingResult {
  bound: boolean;
  alreadyBound: boolean;
  inviteCode: string;
  rewardAmount: number;
  message: string;
}

export interface UserInvitation {
  code: string;
  successfulInvites: number;
  totalReward: number;
  rewardAmount: number;
  afterTime: string;
  enabled: boolean;
  invitedByCode?: string;
  invitedAt?: string;
}

export interface InvitationRecord {
  id: number;
  invitationCodeId: number;
  inviteCode: string;
  inviterUserId: number;
  inviteeUserId: number;
  platform: string;
  externalUserId: string;
  inviteeCreatedAt: string;
  rewardAmount: number;
  rewardStatus: string;
  errorMessage: string;
  rewardedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface SystemUpdateResult {
  started: boolean;
  output: string;
  message: string;
}

export interface Sub2APIGroupRateUpdatePayload {
  rateMultiplier: number;
  groupName?: string;
  createdAt?: string;
}

export interface Sub2APIGroupRateLogUpdatePayload {
  oldRate: number;
  newRate: number;
  createdAt?: string;
  publicVisible?: boolean;
}

export interface Sub2APIGroupRateLogCreatePayload {
  rateMultiplier: number;
  createdAt?: string;
  publicVisible?: boolean;
}

export function getToken() {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY);
}

export function getUserToken() {
  return localStorage.getItem(USER_TOKEN_KEY);
}

export function setUserSession(data: UserLoginResponse) {
  if (data.access_token) {
    localStorage.setItem(USER_TOKEN_KEY, data.access_token);
  }
  if (data.refresh_token) {
    localStorage.setItem(USER_REFRESH_TOKEN_KEY, data.refresh_token);
  }
}

export function clearUserSession() {
  localStorage.removeItem(USER_TOKEN_KEY);
  localStorage.removeItem(USER_REFRESH_TOKEN_KEY);
}

export async function login(username: string, password: string) {
  const data = await request<{ token: string; expiresInHours: number }>('/api/admin/login', {
    method: 'POST',
    body: JSON.stringify({ username, password })
  }, false);
  setToken(data.token);
  return data;
}

export async function userLogin(email: string, password: string) {
  const data = await userRequest<UserLoginResponse>('/api/user/login', {
    method: 'POST',
    body: JSON.stringify({ email, password })
  }, false);
  if (data.access_token) {
    setUserSession(data);
  }
  return data;
}

export async function userLogin2FA(tempToken: string, totpCode: string) {
  const data = await userRequest<UserLoginResponse>('/api/user/login/2fa', {
    method: 'POST',
    body: JSON.stringify({ temp_token: tempToken, totp_code: totpCode })
  }, false);
  if (data.access_token) {
    setUserSession(data);
  }
  return data;
}

export async function fetchCurrentUser() {
  return userRequest<Sub2APIUserProfile>('/api/user/me');
}

export async function fetchUserRechargeRewards() {
  return userRequest<UserRechargeRewards>('/api/user/recharge-rewards');
}

export async function fetchUserCheckInStatus() {
  return userRequest<CheckInResult>('/api/user/check-in');
}

export async function userCheckIn() {
  return userRequest<CheckInResult>('/api/user/check-in', {
    method: 'POST'
  });
}

export async function claimRechargeReward(activityId: number, tierId: number) {
  return userRequest<ClaimRechargeRewardResult>(`/api/user/recharge-rewards/${activityId}/tiers/${tierId}/claim`, {
    method: 'POST'
  });
}

export async function bindSocialAccount(payload: SocialBindingPayload) {
  return userRequest<SocialBindingResult>('/api/user/social-bindings', {
    method: 'POST',
    body: JSON.stringify(payload)
  });
}

export async function fetchStats() {
  return request<Stats>('/api/admin/stats');
}

export async function fetchCheckInSettings() {
  return request<CheckInSettings>('/api/admin/settings/check-in');
}

export async function fetchCheckInStats() {
  return request<CheckInStats>('/api/admin/check-in-stats');
}

export async function updateCheckInSettings(
  dailyMaxUsers: number,
  dailyLimitMode: 'shared' | 'separate',
  directDailyMaxUsers: number,
  socialDailyMaxUsers: number,
  directPrizeTiers: PrizeTierSetting[],
  socialPrizeTiers: PrizeTierSetting[],
  groupLink: string,
  admin: AdminSettings,
  sub2api: Sub2APISettings,
  invitation: InvitationSettings
) {
  return request<CheckInSettings>('/api/admin/settings/check-in', {
    method: 'PUT',
    body: JSON.stringify({ dailyMaxUsers, dailyLimitMode, directDailyMaxUsers, socialDailyMaxUsers, prizeTiers: directPrizeTiers, directPrizeTiers, socialPrizeTiers, groupLink, admin, sub2api, invitation })
  });
}

export async function fetchUserInvitation() {
  return userRequest<UserInvitation>('/api/user/invitation');
}

export async function generateUserInvitationCode() {
  return userRequest<UserInvitation>('/api/user/invitation/code', { method: 'POST' });
}

export async function fetchCodes(params: Record<string, string | number | undefined>) {
  const query = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      query.set(key, String(value));
    }
  });
  return request<PageResult<RedeemCode>>(`/api/admin/codes?${query.toString()}`);
}

export async function createCode(payload: CodePayload) {
  return request<RedeemCode>('/api/admin/codes', {
    method: 'POST',
    body: JSON.stringify(payload)
  });
}

export async function batchImportCodes(payload: BatchImportPayload) {
  return request<BatchImportResult>('/api/admin/codes/batch-import', {
    method: 'POST',
    body: JSON.stringify(payload)
  });
}

export async function updateCode(id: number, payload: CodePayload) {
  return request<RedeemCode>(`/api/admin/codes/${id}`, {
    method: 'PUT',
    body: JSON.stringify(payload)
  });
}

export async function deleteCode(id: number) {
  return request<void>(`/api/admin/codes/${id}`, {
    method: 'DELETE'
  });
}

export async function fetchFavoriteSites(params: Record<string, string | number | undefined>) {
  const query = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      query.set(key, String(value));
    }
  });
  return request<PageResult<FavoriteSite>>(`/api/admin/favorite-sites?${query.toString()}`);
}

export async function fetchFavoriteSiteGroups() {
  return request<string[]>('/api/admin/favorite-sites/groups');
}

export async function createFavoriteSite(payload: FavoriteSitePayload) {
  return request<FavoriteSite>('/api/admin/favorite-sites', {
    method: 'POST',
    body: JSON.stringify(payload)
  });
}

export async function updateFavoriteSite(id: number, payload: FavoriteSitePayload) {
  return request<FavoriteSite>(`/api/admin/favorite-sites/${id}`, {
    method: 'PUT',
    body: JSON.stringify(payload)
  });
}

export async function deleteFavoriteSite(id: number) {
  return request<void>(`/api/admin/favorite-sites/${id}`, {
    method: 'DELETE'
  });
}

export async function fetchRechargeActivities() {
  return request<RechargeActivity[]>('/api/admin/recharge-activities');
}

export async function fetchRechargeRewardClaims(params: Record<string, string | number | undefined>) {
  const query = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      query.set(key, String(value));
    }
  });
  return request<PageResult<AdminRechargeRewardClaim>>(`/api/admin/recharge-reward-claims?${query.toString()}`);
}

export async function fetchInvitationRecords(params: Record<string, string | number | undefined>) {
  const query = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      query.set(key, String(value));
    }
  });
  return request<PageResult<InvitationRecord>>(`/api/admin/invitations?${query.toString()}`);
}

export async function createRechargeActivity(payload: RechargeActivityPayload) {
  return request<RechargeActivity>('/api/admin/recharge-activities', {
    method: 'POST',
    body: JSON.stringify(payload)
  });
}

export async function updateRechargeActivity(id: number, payload: RechargeActivityPayload) {
  return request<RechargeActivity>(`/api/admin/recharge-activities/${id}`, {
    method: 'PUT',
    body: JSON.stringify(payload)
  });
}

export async function deleteRechargeActivity(id: number) {
  return request<void>(`/api/admin/recharge-activities/${id}`, {
    method: 'DELETE'
  });
}

export async function fetchSub2APIGroupRateMonitor() {
  return request<Sub2APIGroupRateMonitor>('/api/admin/sub2api/group-rate-monitor');
}

export async function updateSub2APIGroupRateMonitor(settings: Sub2APIGroupRateMonitorSettings) {
  return request<Sub2APIGroupRateMonitor>('/api/admin/sub2api/group-rate-monitor', {
    method: 'PUT',
    body: JSON.stringify(settings)
  });
}

export async function refreshSub2APIGroupRates() {
  return request<Sub2APIGroupRateMonitor>('/api/admin/sub2api/group-rate-monitor/refresh', {
    method: 'POST'
  });
}

export async function updateSub2APIGroupRate(groupId: string, payload: Sub2APIGroupRateUpdatePayload) {
  return request<Sub2APIGroupRateMonitor>(`/api/admin/sub2api/group-rate-monitor/groups/${encodeURIComponent(groupId)}/rate`, {
    method: 'PUT',
    body: JSON.stringify(payload)
  });
}

export async function updateSub2APIGroupRateLog(id: number, payload: Sub2APIGroupRateLogUpdatePayload) {
  return request<Sub2APIGroupRateMonitor>(`/api/admin/sub2api/group-rate-monitor/logs/${id}`, {
    method: 'PUT',
    body: JSON.stringify(payload)
  });
}

export async function fetchSub2APIGroupRateLogs(groupId: string) {
  return request<Sub2APIGroupRateLog[]>(`/api/admin/sub2api/group-rate-monitor/groups/${encodeURIComponent(groupId)}/logs`);
}

export async function createSub2APIGroupRateLog(groupId: string, payload: Sub2APIGroupRateLogCreatePayload) {
  return request<Sub2APIGroupRateLog[]>(`/api/admin/sub2api/group-rate-monitor/groups/${encodeURIComponent(groupId)}/logs`, {
    method: 'POST',
    body: JSON.stringify(payload)
  });
}

export async function deleteSub2APIGroupRateLog(id: number) {
  return request<Sub2APIGroupRateLog[]>(`/api/admin/sub2api/group-rate-monitor/logs/${id}`, {
    method: 'DELETE'
  });
}

export async function fetchSystemUpdateCheck() {
  return request<SystemUpdateCheck>('/api/admin/system/update-check');
}

export async function runSystemUpdate() {
  return request<SystemUpdateResult>('/api/admin/system/update', {
    method: 'POST'
  });
}

export async function fetchPublicSub2APIGroupRateSeries() {
  return request<Sub2APIGroupRateSeries[]>('/api/public/sub2api/group-rate-series', {}, false);
}

async function request<T>(path: string, init: RequestInit = {}, withAuth = true): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set('Content-Type', 'application/json;charset=UTF-8');

  const token = getToken();
  if (withAuth && token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers
  });

  if (withAuth && response.status === 401) {
    clearToken();
    redirectToLogin();
    throw new Error('登录已过期，请重新登录');
  }

  if (!response.ok) {
    const error = await response.json().catch(() => ({ message: '请求失败' }));
    throw new Error(error.message ?? '请求失败');
  }

  if (response.status === 204 || init.method === 'DELETE') {
    return undefined as T;
  }

  return response.json();
}

async function userRequest<T>(path: string, init: RequestInit = {}, withAuth = true): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set('Content-Type', 'application/json;charset=UTF-8');

  const token = getUserToken();
  if (withAuth && token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers
  });

  if (withAuth && response.status === 401) {
    clearUserSession();
    redirectToLogin();
    throw new Error('用户登录已过期，请重新登录');
  }

  if (!response.ok) {
    const error = await response.json().catch(() => ({ message: '请求失败' }));
    throw new Error(error.message ?? '请求失败');
  }

  if (response.status === 204 || init.method === 'DELETE') {
    return undefined as T;
  }

  return response.json();
}
