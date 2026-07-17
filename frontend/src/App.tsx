import React, { FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import { BrowserRouter, Navigate, Route, Routes, useLocation, useNavigate, useParams } from 'react-router-dom';
import {
  Bookmark,
  CalendarCheck2,
  ChevronDown,
  CheckCircle2,
  CircleDollarSign,
  Copy,
  ExternalLink,
  Gift,
  Globe2,
  GripVertical,
  KeyRound,
  LogOut,
  MessageCircle,
  Pencil,
  Plus,
  Search,
  Send,
  Settings2,
  ShieldCheck,
  Trash2,
  UserRound,
  UserPlus,
  X
} from 'lucide-react';
import {
  AdminSettings,
  batchImportCodes,
  CheckInStats,
  CheckInResult,
  clearUserSession,
  clearToken,
  CodePayload,
  connectTelegramBot,
  createCode,
  createFavoriteSite,
  createRechargeActivity,
  createSub2APIGroupRateLog,
  claimRechargeReward,
  deleteCode,
  deleteFavoriteSite,
  deleteRechargeActivity,
  deleteSub2APIGroupRateLog,
  fetchCheckInSettings,
  fetchCheckInStats,
  fetchCodes,
  fetchCurrentUser,
  fetchFavoriteSiteGroups,
  fetchFavoriteSites,
  fetchInvitationRecords,
  fetchInvitationStats,
  fetchPublicSub2APIGroupRateSeries,
  fetchRechargeActivities,
  fetchRechargeRewardClaims,
  fetchRechargeRewardStats,
  fetchSub2APIGroupRateMonitor,
  fetchSub2APIGroupRateLogs,
  fetchSystemUpdateCheck,
  fetchUserCheckInStatus,
  fetchUserRechargeRewards,
  fetchUserInvitation,
  fetchStats,
  FavoriteSite,
  FavoriteSitePayload,
  getUserToken,
  getToken,
  login,
  PrizeTierSetting,
  RedeemCode,
  RedeemCodeStatus,
  RechargeActivity,
  RechargeActivityPayload,
  RechargeRewardStats,
  Stats,
  InvitationSettings,
  InvitationGuideSettings,
  InvitationRecord,
  InvitationStats,
  Sub2APIGroupRateMonitor,
  Sub2APIGroupRateGroup,
  Sub2APIGroupRateLog,
  Sub2APIGroupRateMonitorSettings,
  Sub2APIGroupRateSeries,
  Sub2APISettings,
  TelegramSettings,
  Sub2APIUserProfile,
  SystemUpdateCheck,
  updateCheckInSettings,
  updateCode,
  updateFavoriteSite,
  updateRechargeActivity,
  updateSub2APIGroupRateMonitor,
  updateSub2APIGroupRateLog,
  refreshSub2APIGroupRates,
  runSystemUpdate,
  UserRechargeRewards,
  userCheckIn,
  userLogin,
  userLogin2FA,
  generateUserInvitationCode,
  UserInvitation,
  AdminRechargeRewardClaim
} from './api';
import {
  DashboardSection,
  emptyAdminSettings,
  emptyCheckInStats,
  emptyGroupRateMonitor,
  emptyInvitationSettings,
  emptyInvitationGuideSettings,
  emptyStats,
  emptySub2APISettings,
  emptyTelegramSettings,
  favoriteEmojiPresets,
  favoriteIconPresets,
  LoginMode,
  statusText
} from './appConstants';
import {
  bindPendingSocialAccount,
  consumeSocialBindingNotice,
  formatDateTime,
  formatOptionalDate,
  formatPlatformName,
  formatToday,
  getPendingSocialBindingFromURL,
  getInviteCodeFromURL,
  isDashboardSection,
  normalizeStats,
  parsePrizeTierDrafts,
  parseRechargeActivityPayload,
  parseSub2APIDraft,
  parseTelegramDraft,
  prizeTierTotal,
  RechargeRewardTierDraft,
  sectionTitle,
  settingsChanged,
  toAmountOptions,
  toDateTimeLocal,
  toPrizeTierDrafts,
  toRechargeTierDrafts,
  toSub2APIDraft,
  toTelegramDraft,
  updatePrizeTierDraft,
  updateSub2APIDraft
} from './appUtils';
import { CheckInTrendChart, InvitationTrendChart, RateLineChart, RechargeRewardTrendChart } from './components/Charts';
import { alertDialog, confirmDialog, FeedbackHost, notifyError, notifySuccess } from './components/Feedback';


export default function App() {
  return (
    <BrowserRouter>
      <FeedbackHost />
      <Routes>
        <Route path="/" element={<RootRedirect />} />
        <Route path="/login" element={<LoginRoute />} />
        <Route path="/user" element={<UserRoute />} />
        <Route path="/admin" element={<Navigate to="/admin/recharge" replace />} />
        <Route path="/admin/:section" element={<AdminRoute />} />
        <Route path="*" element={<RootRedirect />} />
      </Routes>
    </BrowserRouter>
  );
}

function RootRedirect() {
  const { search } = useLocation();
  const params = new URLSearchParams(search);
  if (params.get('platform') || params.get('userid') || params.get('invitecode')) {
    return <Navigate to={`/login${search}`} replace />;
  }
  if (getUserToken()) return <Navigate to="/user" replace />;
  if (getToken()) return <Navigate to="/admin/recharge" replace />;
  return <Navigate to="/login" replace />;
}

function LoginRoute() {
  const navigate = useNavigate();
  return (
    <UnifiedLogin
      onAdminLogin={() => navigate('/admin/recharge', { replace: true })}
      onUserLogin={() => navigate('/user', { replace: true })}
    />
  );
}

function UserRoute() {
  const navigate = useNavigate();
  if (!getUserToken()) {
    return <Navigate to="/login" replace />;
  }
  return <UserDashboard onLogout={() => navigate('/login', { replace: true })} />;
}

function AdminRoute() {
  const navigate = useNavigate();
  const { section } = useParams();
  if (!getToken()) {
    return <Navigate to="/login" replace />;
  }
  if (!isDashboardSection(section)) {
    return <Navigate to="/admin/recharge" replace />;
  }
  return <Dashboard section={section} onSectionChange={(nextSection) => navigate(`/admin/${nextSection}`)} onLogout={() => navigate('/login', { replace: true })} />;
}

function UnifiedLogin({ onAdminLogin, onUserLogin }: { onAdminLogin: () => void; onUserLogin: () => void }) {
  const pendingSocialBinding = useMemo(() => getPendingSocialBindingFromURL(), []);
  const lockedInviteCode = useMemo(() => getInviteCodeFromURL(), []);
  const [mode, setMode] = useState<LoginMode>('user');
  const [username, setUsername] = useState('admin');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [totpCode, setTotpCode] = useState('');
  const [tempToken, setTempToken] = useState('');
  const [maskedEmail, setMaskedEmail] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  async function submit(event: FormEvent) {
    event.preventDefault();
    setLoading(true);
    setError('');
    try {
      if (mode === 'admin') {
        await login(username, password);
        clearUserSession();
        onAdminLogin();
        return;
      }

      const data = tempToken
        ? await userLogin2FA(tempToken, totpCode)
        : await userLogin(email, password);
      if (data.requires_2fa && data.temp_token) {
        setTempToken(data.temp_token);
        setMaskedEmail(data.user_email_masked ?? email);
        setPassword('');
        setTotpCode('');
        return;
      }
      if (data.access_token) {
        clearToken();
        await bindPendingSocialAccount(pendingSocialBinding
          ? { ...pendingSocialBinding, ...(lockedInviteCode ? { inviteCode: lockedInviteCode } : {}) }
          : null);
        onUserLogin();
        return;
      }
      notifyError('登录未返回有效的用户令牌');
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '登录失败');
    } finally {
      setLoading(false);
    }
  }

  function switchMode(nextMode: LoginMode) {
    setMode(nextMode);
    setError('');
    setTempToken('');
    setTotpCode('');
    setMaskedEmail('');
  }

  return (
    <main className="login-shell">
      <section className="login-panel dual-login-panel">
        <div className="login-panel-head">
          <div className="brand-mark">
            {mode === 'user' ? <UserRound size={26} /> : <ShieldCheck size={26} />}
          </div>
          <div className="login-mode-toggle" aria-label="选择登录类型">
            <button type="button" className={mode === 'user' ? 'is-active' : ''} onClick={() => switchMode('user')}>
              <UserRound size={16} />
              用户
            </button>
            <button type="button" className={mode === 'admin' ? 'is-active' : ''} onClick={() => switchMode('admin')}>
              <ShieldCheck size={16} />
              管理员
            </button>
          </div>
        </div>
        <h1>{mode === 'user' ? '用户登录' : '管理员后台'}</h1>
        <p>{mode === 'user' ? '使用 Sub2API 账号密码登录，进入你的专属页面。' : '管理员登录后可维护签到、兑换码和系统设置。'}</p>
        {mode === 'user' && pendingSocialBinding && (
          <div className="social-bind-hint">
            登录后将绑定 {pendingSocialBinding.platform} 账号 {pendingSocialBinding.userId}
          </div>
        )}
        <form onSubmit={submit} className="login-form">
          {mode === 'admin' ? (
            <>
              <label>
                管理员账号
                <input value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" />
              </label>
              <label>
                管理员密码
                <input
                  type="password"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  autoComplete="current-password"
                />
              </label>
            </>
          ) : tempToken ? (
            <>
              <div className="success-line">已验证密码，请输入 {maskedEmail || '当前账号'} 的 2FA 验证码。</div>
              <label>
                2FA 验证码
                <input
                  value={totpCode}
                  onChange={(event) => setTotpCode(event.target.value)}
                  autoComplete="one-time-code"
                  inputMode="numeric"
                  maxLength={6}
                />
              </label>
            </>
          ) : (
            <>
              <label>
                Sub2API 邮箱
                <input
                  type="email"
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  autoComplete="email"
                  placeholder="you@example.com"
                />
              </label>
              <label>
                Sub2API 密码
                <input
                  type="password"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  autoComplete="current-password"
                />
              </label>
              {lockedInviteCode && (
                <label>
                  邀请码
                  <input value={lockedInviteCode} readOnly aria-readonly="true" className="locked-invite-input" />
                </label>
              )}
            </>
          )}
          {error && <div className="error-line">{error}</div>}
          <button type="submit" disabled={loading}>
            {loading ? '登录中...' : tempToken ? '验证并进入' : '登录'}
          </button>
        </form>
      </section>
    </main>
  );
}

function UserDashboard({ onLogout }: { onLogout: () => void }) {
  const [user, setUser] = useState<Sub2APIUserProfile | null>(null);
  const [rewards, setRewards] = useState<UserRechargeRewards | null>(null);
  const [checkInStatus, setCheckInStatus] = useState<CheckInResult | null>(null);
  const [invitation, setInvitation] = useState<UserInvitation | null>(null);
  const [invitationGuidePlatform, setInvitationGuidePlatform] = useState<'qq' | 'telegram'>('qq');
  const [generatingInviteCode, setGeneratingInviteCode] = useState(false);
  const [loading, setLoading] = useState(true);
  const [claimingTierId, setClaimingTierId] = useState<number | null>(null);
  const [checkingIn, setCheckingIn] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState(() => consumeSocialBindingNotice());
  const [publicRateSeries, setPublicRateSeries] = useState<Sub2APIGroupRateSeries[]>([]);

  async function loadUser() {
    setLoading(true);
    setError('');
    try {
      const [nextUser, nextRewards, nextCheckInStatus, nextInvitation] = await Promise.all([
        fetchCurrentUser(),
        fetchUserRechargeRewards(),
        fetchUserCheckInStatus(),
        fetchUserInvitation()
      ]);
      setUser(nextUser);
      setRewards(nextRewards);
      setCheckInStatus(nextCheckInStatus);
      setInvitation(nextInvitation);
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '加载用户信息失败');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadUser();
    fetchPublicSub2APIGroupRateSeries()
      .then((series) => setPublicRateSeries(Array.isArray(series) ? series : []))
      .catch(() => setPublicRateSeries([]));
  }, []);

  useEffect(() => {
    if (success) {
      notifySuccess(success);
    }
  }, [success]);

  function logout() {
    clearUserSession();
    onLogout();
  }

  async function checkIn() {
    setCheckingIn(true);
    setError('');
    setSuccess('');
    try {
      const result = await userCheckIn();
      setCheckInStatus(result);
      notifySuccess(result.alreadyCheckedIn
        ? '今日已签到'
        : `签到成功，${Number(result.amount).toFixed(2)} 余额已自动入账`);
      await loadUser();
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '签到失败');
    } finally {
      setCheckingIn(false);
    }
  }

  async function claim(activityId: number, tierId: number) {
    setClaimingTierId(tierId);
    setError('');
    setSuccess('');
    try {
      const result = await claimRechargeReward(activityId, tierId);
      notifySuccess(`已领取 ${Number(result.rewardAmount).toFixed(2)} 余额奖励`);
      await loadUser();
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '领取失败');
    } finally {
      setClaimingTierId(null);
    }
  }

  async function generateInviteCode() {
    setGeneratingInviteCode(true);
    try {
      const nextInvitation = await generateUserInvitationCode();
      setInvitation(nextInvitation);
      notifySuccess('专属邀请码已生成');
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '生成邀请码失败');
    } finally {
      setGeneratingInviteCode(false);
    }
  }

  async function copyInviteCode() {
    if (!invitation?.code) return;
    try {
      const telegramInviteURL = invitation.guides?.telegram?.inviteUrl?.trim() ?? '';
      const copyTelegramLink = invitationGuidePlatform === 'telegram' && telegramInviteURL;
      await navigator.clipboard.writeText(copyTelegramLink || invitation.code);
      notifySuccess(copyTelegramLink ? 'Telegram 邀请链接已复制' : '邀请码已复制');
    } catch {
      notifyError('复制失败，请手动复制邀请码');
    }
  }

  const displayName = user?.username || user?.email || 'Sub2API 用户';
  const balance = typeof user?.balance === 'number' ? user.balance.toFixed(2) : '-';
  const totalRecharged = typeof rewards?.totalRecharged === 'number'
    ? rewards.totalRecharged.toFixed(2)
    : (typeof user?.total_recharged === 'number' ? user.total_recharged.toFixed(2) : '-');
  const rewardActivities = rewards?.activities ?? [];
  const rewardMilestones = rewardActivities
    .flatMap((activity) => activity.tiers.map((tier) => ({ activity, tier })))
    .sort((left, right) => Number(left.tier.thresholdAmount) - Number(right.tier.thresholdAmount));
  const rewardMaxThreshold = Math.max(
    ...rewardMilestones.map(({ tier }) => Number(tier.thresholdAmount)),
    1
  );
  const rewardProgressPercent = Math.min(
    100,
    Math.max(0, (Number(rewards?.totalRecharged ?? 0) / rewardMaxThreshold) * 100)
  );
  const checkInAmount = checkInStatus?.code ? Number(checkInStatus.amount).toFixed(2) : '-';
  const checkInDate = checkInStatus?.signDate ?? formatToday();
  const checkInGroupLink = checkInStatus?.groupLink?.trim() ?? '';
  const publicSocialPrizeTiers = Array.isArray(checkInStatus?.socialPrizeTiers) ? checkInStatus.socialPrizeTiers : [];
  const socialBindings = Array.isArray(user?.socialBindings) ? user.socialBindings : [];
  const qqInvitationGuide = invitation?.guides?.qq ?? {
    platform: 'qq' as const,
    groupNumber: '799128896',
    groupLink: checkInGroupLink,
    botMention: '@咕咕嘎嘎',
    rewardAmount: invitation?.rewardAmount ?? 0,
    afterTime: invitation?.afterTime ?? '',
    enabled: invitation?.enabled ?? false
  };
  const telegramInvitationGuide = invitation?.guides?.telegram ?? {
    platform: 'telegram' as const,
    groupLink: '',
    botUsername: '',
    inviteUrl: '',
    membershipCheckEnabled: false,
    rewardAmount: invitation?.rewardAmount ?? 0,
    afterTime: invitation?.afterTime ?? '',
    enabled: invitation?.enabled ?? false
  };
  const activeInvitationGuide = invitationGuidePlatform === 'telegram' ? telegramInvitationGuide : qqInvitationGuide;
  const activeInvitationReward = Number(activeInvitationGuide.rewardAmount ?? 0);

  return (
    <main className="user-layout">
      <header className="user-topbar">
        <div className="user-brand">
          <div className="brand-mark compact">
            <UserRound size={21} />
          </div>
          <div>
            <span className="eyebrow">User Center</span>
            <h1>用户专属页面</h1>
          </div>
        </div>
        <button className="ghost-btn" onClick={logout} type="button">
          <LogOut size={18} />
          退出
        </button>
      </header>

      {error && <div className="error-banner">{error}</div>}

      <section className="user-hero">
        <div>
          <span className="eyebrow">Sub2API Account</span>
          <h2>{loading ? '正在加载...' : displayName}</h2>
          <p>{user?.email || '登录后可查看你的账户余额、并发额度和账号状态。'}</p>
        </div>
        <button className="ghost-btn" type="button" onClick={loadUser} disabled={loading}>
          <CheckCircle2 size={17} />
          {loading ? '刷新中...' : '刷新'}
        </button>
      </section>

      <section className="user-metric-grid">
        <article className="metric metric-green">
          <span>余额</span>
          <strong>{balance}</strong>
        </article>
        <article className="metric metric-blue">
          <span>并发额度</span>
          <strong>{user?.concurrency ?? '-'}</strong>
        </article>
        <article className="metric metric-ink">
          <span>账号状态</span>
          <strong>{user?.status || '-'}</strong>
        </article>
        <article className="metric metric-red">
          <span>累计充值</span>
          <strong>{totalRecharged}</strong>
        </article>
      </section>

      <section className="user-checkin-panel">
        <div>
          <span className="eyebrow">Daily Check-in</span>
          <h2>{checkInStatus?.alreadyCheckedIn ? '今日已签到' : '今日签到'}</h2>
          <p>{checkInStatus?.alreadyCheckedIn ? '今天的奖励已发放，明天再来领取新的签到奖励。' : '点击后将使用当前登录账号签到，奖励余额会自动入账。'}</p>
        </div>
        <div className="user-checkin-result">
          <div>
            <span>签到日期</span>
            <strong>{checkInDate}</strong>
          </div>
          <div>
            <span>奖励金额</span>
            <strong>{checkInAmount}</strong>
          </div>
          <button
            className="primary-btn"
            type="button"
            onClick={checkIn}
            disabled={loading || checkingIn || checkInStatus?.alreadyCheckedIn}
          >
            <CalendarCheck2 size={18} />
            {checkingIn ? '签到中...' : checkInStatus?.alreadyCheckedIn ? '已签到' : '立即签到'}
          </button>
        </div>
        {checkInStatus?.code && (
          <div className="checkin-code-line">
            <span>入账凭证</span>
            <strong>{checkInStatus.code}</strong>
          </div>
        )}
        <div className="checkin-note-line">
          <span>站内签到按当前规则发放，群内签到奖励概率公开如下。</span>
          {checkInGroupLink ? (
            <a href={checkInGroupLink} target="_blank" rel="noreferrer">
              <ExternalLink size={15} />
              群链接
            </a>
          ) : (
            <strong>群链接暂未设置</strong>
          )}
        </div>
        <div className="checkin-probability-panel">
          <div className="checkin-probability-head">
            <div>
              <CircleDollarSign size={17} />
              <strong>群内签到金额概率</strong>
            </div>
            <span>合计 {publicSocialPrizeTiers.reduce((total, tier) => total + Number(tier.probability || 0), 0).toFixed(2)}%</span>
          </div>
          {publicSocialPrizeTiers.length > 0 ? (
            <div className="checkin-probability-list">
              {publicSocialPrizeTiers.map((tier) => {
                const probability = Number(tier.probability || 0);
                return (
                  <div className="checkin-probability-row" key={`${tier.amount}-${tier.probability}`}>
                    <strong>{Number(tier.amount).toFixed(2)} 余额</strong>
                    <div className="probability-track" aria-hidden="true">
                      <span style={{ width: `${Math.min(100, Math.max(0, probability))}%` }} />
                    </div>
                    <span>{probability.toFixed(2)}%</span>
                  </div>
                );
              })}
            </div>
          ) : (
            <div className="amount-stats-empty">群内签到概率暂未配置</div>
          )}
        </div>
      </section>

      <section className="user-invitation-panel">
        <div className="invitation-panel-head">
          <div>
            <span className="eyebrow">Invitation</span>
            <h2>邀请新用户</h2>
            <p>{invitationGuidePlatform === 'telegram'
              ? '将专属 Telegram 邀请链接发送给新人，由 Bot 校验身份并返回网页登录绑定链接。'
              : `将专属邀请码发送到 QQ 群 ${qqInvitationGuide.groupNumber || '未配置'}，由新人通过机器人返回的绑定链接完成邀请。`}</p>
          </div>
          <div className={`invitation-status ${activeInvitationGuide.enabled ? 'is-active' : ''}`}>
            {activeInvitationGuide.enabled ? `每位奖励 ${activeInvitationReward.toFixed(2)}` : '活动暂未启用'}
          </div>
        </div>
        <div className="invitation-code-row">
          <div className="invitation-code-box">
            <span>我的邀请码</span>
            <strong>{invitation?.code || '尚未生成'}</strong>
          </div>
          {invitation?.code ? (
            <button className="ghost-btn" type="button" onClick={copyInviteCode} title="复制邀请码">
              <Copy size={17} />
              {invitationGuidePlatform === 'telegram' && telegramInvitationGuide.inviteUrl ? '复制邀请链接' : '复制邀请码'}
            </button>
          ) : (
            <button className="primary-btn" type="button" onClick={generateInviteCode} disabled={loading || generatingInviteCode}>
              <Gift size={18} />
              {generatingInviteCode ? '生成中...' : '生成邀请码'}
            </button>
          )}
        </div>
        <div className="invitation-guide">
          <div className="invitation-guide-toolbar">
            <div className="invitation-guide-head">
              <UserPlus size={17} />
              <strong>如何完成一次有效邀请</strong>
            </div>
            <div className="invitation-platform-switch" role="tablist" aria-label="邀请平台">
              <button
                type="button"
                role="tab"
                aria-selected={invitationGuidePlatform === 'qq'}
                className={invitationGuidePlatform === 'qq' ? 'is-active' : ''}
                onClick={() => setInvitationGuidePlatform('qq')}
              >
                <MessageCircle size={15} />
                QQ
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={invitationGuidePlatform === 'telegram'}
                className={invitationGuidePlatform === 'telegram' ? 'is-active' : ''}
                onClick={() => setInvitationGuidePlatform('telegram')}
              >
                <Send size={15} />
                Telegram
              </button>
            </div>
          </div>
          {invitationGuidePlatform === 'qq' ? (
            <ol>
              <li>
                <span>1</span>
                <div>让新人加入 QQ 群 <strong>{qqInvitationGuide.groupNumber || '群号未配置'}</strong>{qqInvitationGuide.groupLink && <>，<a href={qqInvitationGuide.groupLink} target="_blank" rel="noreferrer">打开群链接</a></>}。</div>
              </li>
              <li>
                <span>2</span>
                <div>新人先注册 Sub2API 账号，账号创建时间必须晚于 <strong>{formatOptionalDate(qqInvitationGuide.afterTime)}</strong>。</div>
              </li>
              <li>
                <span>3</span>
                <div>新人在群里发送 <code>{qqInvitationGuide.botMention || '机器人称呼未配置'} 绑定{invitation?.code || '你的邀请码'}</code>。</div>
              </li>
              <li>
                <span>4</span>
                <div>机器人会返回专属绑定链接，链接包含平台账号和邀请码，请勿转发给其他人。</div>
              </li>
              <li>
                <span>5</span>
                <div>新人点击链接并成功登录一次即完成绑定；成功后邀请人获得 <strong>{activeInvitationReward.toFixed(2)} 余额</strong>。</div>
              </li>
            </ol>
          ) : (
            <ol>
              <li>
                <span>1</span>
                <div>让新人先加入指定 Telegram 群组{telegramInvitationGuide.groupLink
                  ? <>，<a href={telegramInvitationGuide.groupLink} target="_blank" rel="noreferrer">打开群链接</a></>
                  : <>，<strong>群链接暂未配置</strong></>}。</div>
              </li>
              <li>
                <span>2</span>
                <div>新人先注册 Sub2API 账号，账号创建时间必须晚于 <strong>{formatOptionalDate(telegramInvitationGuide.afterTime)}</strong>。</div>
              </li>
              <li>
                <span>3</span>
                <div>将专属邀请链接发送给新人：{telegramInvitationGuide.inviteUrl
                  ? <a className="invitation-deep-link" href={telegramInvitationGuide.inviteUrl} target="_blank" rel="noreferrer">{telegramInvitationGuide.inviteUrl}</a>
                  : <strong>连接 Telegram Bot 并生成邀请码后显示</strong>}。</div>
              </li>
              <li>
                <span>4</span>
                <div>新人打开链接并点击 Start；Bot {telegramInvitationGuide.membershipCheckEnabled ? '确认已加入指定群组后' : '识别 Telegram 身份后'}返回专属网页登录绑定链接。</div>
              </li>
              <li>
                <span>5</span>
                <div>新人登录 Sub2API 账号完成绑定；成功后邀请人获得 <strong>{activeInvitationReward.toFixed(2)} 余额</strong>。</div>
              </li>
            </ol>
          )}
        </div>
        <div className="invitation-metrics">
          <div><span>成功邀请</span><strong>{invitation?.successfulInvites ?? 0}</strong></div>
          <div><span>累计奖励</span><strong>{Number(invitation?.totalReward ?? 0).toFixed(2)}</strong></div>
          <div><span>新人时间门槛</span><strong>{formatOptionalDate(activeInvitationGuide.afterTime)}</strong></div>
        </div>
        {invitation?.invitedByCode && (
          <div className="invitation-bound-note">
            <UserPlus size={16} />
            当前账号已通过邀请码 {invitation.invitedByCode} 完成邀请绑定
          </div>
        )}
      </section>

      <section className="user-info-panel recharge-reward-panel">
        <div className="settings-title">
          <CircleDollarSign size={18} />
          <span>累计充值活动</span>
        </div>
        <div className="user-reward-list">
          {rewardMilestones.length > 0 && (
            <article className="user-reward-activity">
              <div className="user-reward-head">
                <div>
                  <strong>累计充值总进度</strong>
                  <p>所有活动档位共用一条进度线，按达标金额从低到高领取。</p>
                </div>
                <div className="reward-progress-meta">
                  <span>当前 {totalRecharged}</span>
                  <span>最高奖励 {rewardMaxThreshold.toFixed(2)}</span>
                </div>
              </div>

              <div className="reward-progress" aria-label="累计充值活动总进度">
                <div className="reward-progress-track">
                  <div className="reward-progress-fill" style={{ width: `${rewardProgressPercent}%` }} />
                  {rewardMilestones.map(({ activity, tier }) => {
                    const markerLeft = Math.min(
                      100,
                      Math.max(0, (Number(tier.thresholdAmount) / rewardMaxThreshold) * 100)
                    );
                    const markerEdge = markerLeft < 12 ? 'is-start' : markerLeft > 88 ? 'is-end' : '';
                    return (
                      <div
                        className={`reward-progress-point ${markerEdge} ${
                          tier.claimed ? 'is-claimed' : tier.eligible ? 'is-ready' : ''
                        }`}
                        key={`${activity.id}-${tier.id}`}
                        style={{ left: `${markerLeft}%` }}
                        tabIndex={0}
                        aria-label={`${activity.name} 满 ${Number(tier.thresholdAmount).toFixed(2)} 奖励 ${Number(tier.rewardAmount).toFixed(2)}`}
                      >
                        <span className="reward-progress-marker" />
                        <div className="reward-progress-popover">
                          <span className="reward-activity-name">{activity.name}</span>
                          <span>满 {Number(tier.thresholdAmount).toFixed(2)}</span>
                          <strong>奖励 {Number(tier.rewardAmount).toFixed(2)}</strong>
                          {tier.claimed ? (
                            <span className="reward-status claimed">已领取</span>
                          ) : (
                            <button
                              className="ghost-btn"
                              type="button"
                              disabled={!tier.eligible || claimingTierId === tier.id}
                              onClick={() => claim(activity.id, tier.id)}
                            >
                              {claimingTierId === tier.id ? '领取中...' : tier.eligible ? '领取' : '未达标'}
                            </button>
                          )}
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            </article>
          )}
          {!loading && rewardMilestones.length === 0 && (
            <div className="amount-stats-empty">暂无可参与的累计充值活动</div>
          )}
        </div>
      </section>

      {publicRateSeries.length > 0 && (
        <section className="user-info-panel">
          <div className="settings-title">
            <Globe2 size={18} />
            <span>公开分组倍率</span>
          </div>
          <RateLineChart series={publicRateSeries} />
        </section>
      )}

      <section className="user-info-panel">
        <div className="settings-title">
          <KeyRound size={18} />
          <span>账户信息</span>
        </div>
        <dl className="user-info-list">
          <div>
            <dt>用户 ID</dt>
            <dd>{user?.id ?? '-'}</dd>
          </div>
          <div>
            <dt>邮箱</dt>
            <dd>{user?.email || '-'}</dd>
          </div>
          <div>
            <dt>用户名</dt>
            <dd>{user?.username || '-'}</dd>
          </div>
          <div>
            <dt>角色</dt>
            <dd>{user?.role || '-'}</dd>
          </div>
        </dl>

        <div className="social-binding-section">
          <div className="social-binding-head">
            <strong>已绑定平台</strong>
            <span>{socialBindings.length} 个</span>
          </div>
          {loading ? (
            <div className="amount-stats-empty">正在加载绑定平台</div>
          ) : socialBindings.length > 0 ? (
            <div className="social-binding-list">
              {socialBindings.map((binding) => (
                <article className="social-binding-item" key={binding.id}>
                  <div>
                    <span>平台</span>
                    <strong>{formatPlatformName(binding.platform)}</strong>
                  </div>
                  <div>
                    <span>平台用户 ID</span>
                    <strong>{binding.externalUserId}</strong>
                  </div>
                  <div>
                    <span>绑定时间</span>
                    <strong>{formatOptionalDate(binding.createdAt)}</strong>
                  </div>
                </article>
              ))}
            </div>
          ) : (
            <div className="amount-stats-empty">暂无绑定平台</div>
          )}
        </div>
      </section>
    </main>
  );
}

function Dashboard({
  section: activeSection,
  onSectionChange,
  onLogout
}: {
  section: DashboardSection;
  onSectionChange: (section: DashboardSection) => void;
  onLogout: () => void;
}) {
  const [codes, setCodes] = useState<RedeemCode[]>([]);
  const [stats, setStats] = useState<Stats>(emptyStats);
  const [checkInStats, setCheckInStats] = useState<CheckInStats>(emptyCheckInStats);
  const [keyword, setKeyword] = useState('');
  const [status, setStatus] = useState('');
  const [page, setPage] = useState(0);
  const [totalPages, setTotalPages] = useState(1);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [editing, setEditing] = useState<RedeemCode | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [favoriteSites, setFavoriteSites] = useState<FavoriteSite[]>([]);
  const [draggedFavoriteId, setDraggedFavoriteId] = useState<number | null>(null);
  const favoriteSitesRef = useRef<FavoriteSite[]>([]);
  const favoriteCardRefs = useRef(new Map<number, HTMLElement>());
  const favoriteOrderChangedRef = useRef(false);
  const [favoriteGroups, setFavoriteGroups] = useState<string[]>([]);
  const [favoriteKeyword, setFavoriteKeyword] = useState('');
  const [favoriteGroup, setFavoriteGroup] = useState('');
  const [favoritePage, setFavoritePage] = useState(0);
  const [favoriteTotalPages, setFavoriteTotalPages] = useState(1);
  const [editingFavorite, setEditingFavorite] = useState<FavoriteSite | null>(null);
  const [favoriteModalOpen, setFavoriteModalOpen] = useState(false);
  const [rechargeActivities, setRechargeActivities] = useState<RechargeActivity[]>([]);
  const [rechargeClaims, setRechargeClaims] = useState<AdminRechargeRewardClaim[]>([]);
  const [rechargeRewardStats, setRechargeRewardStats] = useState<RechargeRewardStats | null>(null);
  const [rechargeClaimKeyword, setRechargeClaimKeyword] = useState('');
  const [rechargeClaimStatus, setRechargeClaimStatus] = useState('');
  const [rechargeClaimPage, setRechargeClaimPage] = useState(0);
  const [rechargeClaimTotalPages, setRechargeClaimTotalPages] = useState(1);
  const [editingRechargeActivity, setEditingRechargeActivity] = useState<RechargeActivity | null>(null);
  const [rechargeModalOpen, setRechargeModalOpen] = useState(false);
  const [invitationRecords, setInvitationRecords] = useState<InvitationRecord[]>([]);
  const [invitationStats, setInvitationStats] = useState<InvitationStats | null>(null);
  const [invitationRecordKeyword, setInvitationRecordKeyword] = useState('');
  const [invitationRecordStatus, setInvitationRecordStatus] = useState('');
  const [invitationRecordPage, setInvitationRecordPage] = useState(0);
  const [invitationRecordTotalPages, setInvitationRecordTotalPages] = useState(1);
  const [invitationRecordTotal, setInvitationRecordTotal] = useState(0);
  const [dailyMaxUsers, setDailyMaxUsers] = useState(0);
  const [dailyMaxUsersDraft, setDailyMaxUsersDraft] = useState('');
  const [dailyLimitMode, setDailyLimitMode] = useState<'shared' | 'separate'>('shared');
  const [dailyLimitModeDraft, setDailyLimitModeDraft] = useState<'shared' | 'separate'>('shared');
  const [directDailyMaxUsers, setDirectDailyMaxUsers] = useState(0);
  const [directDailyMaxUsersDraft, setDirectDailyMaxUsersDraft] = useState('');
  const [socialDailyMaxUsers, setSocialDailyMaxUsers] = useState(0);
  const [socialDailyMaxUsersDraft, setSocialDailyMaxUsersDraft] = useState('');
  const [prizeTiers, setPrizeTiers] = useState<PrizeTierSetting[]>([]);
  const [prizeTierDrafts, setPrizeTierDrafts] = useState([{ amount: '1.00', probability: '100.00' }]);
  const [socialPrizeTiers, setSocialPrizeTiers] = useState<PrizeTierSetting[]>([]);
  const [socialPrizeTierDrafts, setSocialPrizeTierDrafts] = useState([{ amount: '1.00', probability: '100.00' }]);
  const [groupLink, setGroupLink] = useState('');
  const [groupLinkDraft, setGroupLinkDraft] = useState('');
  const [frontendPublicUrl, setFrontendPublicUrl] = useState('');
  const [frontendPublicUrlDraft, setFrontendPublicUrlDraft] = useState('');
  const [adminSettings, setAdminSettings] = useState<AdminSettings>(emptyAdminSettings);
  const [adminSettingsDraft, setAdminSettingsDraft] = useState<AdminSettings>(emptyAdminSettings);
  const [sub2api, setSub2api] = useState<Sub2APISettings>(emptySub2APISettings);
  const [sub2apiDraft, setSub2apiDraft] = useState<Sub2APISettings>(emptySub2APISettings);
  const [telegram, setTelegram] = useState<TelegramSettings>(emptyTelegramSettings);
  const [telegramDraft, setTelegramDraft] = useState<TelegramSettings>(emptyTelegramSettings);
  const [telegramConnecting, setTelegramConnecting] = useState(false);
  const [invitationSettings, setInvitationSettings] = useState<InvitationSettings>(emptyInvitationSettings);
  const [invitationSettingsDraft, setInvitationSettingsDraft] = useState<InvitationSettings>(emptyInvitationSettings);
  const [invitationGuide, setInvitationGuide] = useState<InvitationGuideSettings>(emptyInvitationGuideSettings);
  const [invitationGuideDraft, setInvitationGuideDraft] = useState<InvitationGuideSettings>(emptyInvitationGuideSettings);
  const [settingsSaving, setSettingsSaving] = useState(false);
  const [settingsSaved, setSettingsSaved] = useState(false);
  const [groupRateMonitor, setGroupRateMonitor] = useState<Sub2APIGroupRateMonitor>(emptyGroupRateMonitor);
  const [groupRateDraft, setGroupRateDraft] = useState<Sub2APIGroupRateMonitorSettings>(emptyGroupRateMonitor.settings);
  const [groupRateSaving, setGroupRateSaving] = useState(false);
  const [groupRateRefreshing, setGroupRateRefreshing] = useState(false);
  const [groupRateSaved, setGroupRateSaved] = useState(false);
  const [groupRateLogDrafts, setGroupRateLogDrafts] = useState<Record<number, { oldRate: string; newRate: string; createdAt: string; publicVisible: boolean }>>({});
  const [groupRateEditingKey, setGroupRateEditingKey] = useState<string | null>(null);
  const [editingGroupRate, setEditingGroupRate] = useState<Sub2APIGroupRateGroup | null>(null);
  const [systemUpdate, setSystemUpdate] = useState<SystemUpdateCheck | null>(null);
  const [systemUpdateChecking, setSystemUpdateChecking] = useState(false);
  const [systemUpdating, setSystemUpdating] = useState(false);
  const [systemUpdateOutput, setSystemUpdateOutput] = useState('');

  async function load(nextPage = page) {
    setLoading(true);
    setError('');
    try {
      const [statsResult, codesResult, settingsResult] = await Promise.allSettled([
        fetchStats(),
        fetchCodes({ keyword, status, page: nextPage, size: 10 }),
        fetchCheckInSettings()
      ]);
      if (statsResult.status === 'fulfilled') {
        setStats(normalizeStats(statsResult.value));
      }
      if (codesResult.status === 'fulfilled') {
        setCodes(Array.isArray(codesResult.value.content) ? codesResult.value.content : []);
        setPage(codesResult.value.number);
        setTotalPages(Math.max(codesResult.value.totalPages, 1));
      }
      if (settingsResult.status === 'fulfilled') {
        applyCheckInSettings(settingsResult.value);
      }

      const failed = [statsResult, codesResult, settingsResult].find((result) => result.status === 'rejected');
      if (failed?.status === 'rejected') {
        throw failed.reason;
      }
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }

  function applyCheckInSettings(settingsData: Awaited<ReturnType<typeof fetchCheckInSettings>>) {
    setDailyMaxUsers(settingsData.dailyMaxUsers);
    setDailyMaxUsersDraft(String(settingsData.dailyMaxUsers));
    const nextLimitMode = settingsData.dailyLimitMode === 'separate' ? 'separate' : 'shared';
    const nextDirectDailyMaxUsers = Number.isFinite(Number(settingsData.directDailyMaxUsers)) ? Number(settingsData.directDailyMaxUsers) : settingsData.dailyMaxUsers;
    const nextSocialDailyMaxUsers = Number.isFinite(Number(settingsData.socialDailyMaxUsers)) ? Number(settingsData.socialDailyMaxUsers) : settingsData.dailyMaxUsers;
    setDailyLimitMode(nextLimitMode);
    setDailyLimitModeDraft(nextLimitMode);
    setDirectDailyMaxUsers(nextDirectDailyMaxUsers);
    setDirectDailyMaxUsersDraft(String(nextDirectDailyMaxUsers));
    setSocialDailyMaxUsers(nextSocialDailyMaxUsers);
    setSocialDailyMaxUsersDraft(String(nextSocialDailyMaxUsers));
    const directTiers = Array.isArray(settingsData.directPrizeTiers) ? settingsData.directPrizeTiers : settingsData.prizeTiers;
    const socialTiers = Array.isArray(settingsData.socialPrizeTiers) ? settingsData.socialPrizeTiers : directTiers;
    setPrizeTiers(Array.isArray(directTiers) ? directTiers : []);
    setPrizeTierDrafts(toPrizeTierDrafts(directTiers));
    setSocialPrizeTiers(Array.isArray(socialTiers) ? socialTiers : []);
    setSocialPrizeTierDrafts(toPrizeTierDrafts(socialTiers));
    setGroupLink(settingsData.groupLink ?? '');
    setGroupLinkDraft(settingsData.groupLink ?? '');
    setFrontendPublicUrl(settingsData.frontendPublicUrl ?? '');
    setFrontendPublicUrlDraft(settingsData.frontendPublicUrl ?? '');
    const nextAdmin = settingsData.admin ?? emptyAdminSettings;
    setAdminSettings(nextAdmin);
    setAdminSettingsDraft({ ...nextAdmin, password: '' });
    setSub2api(settingsData.sub2api ?? emptySub2APISettings);
    setSub2apiDraft(toSub2APIDraft(settingsData.sub2api ?? emptySub2APISettings));
    const nextTelegram = settingsData.telegram ?? emptyTelegramSettings;
    setTelegram(nextTelegram);
    setTelegramDraft(toTelegramDraft(nextTelegram));
    const nextInvitation = settingsData.invitation ?? emptyInvitationSettings;
    const invitationDraft = {
      ...nextInvitation,
      afterTime: toDateTimeLocal(nextInvitation.afterTime)
    };
    setInvitationSettings(invitationDraft);
    setInvitationSettingsDraft(invitationDraft);
    const nextInvitationGuide = settingsData.invitationGuide ?? emptyInvitationGuideSettings;
    setInvitationGuide(nextInvitationGuide);
    setInvitationGuideDraft(nextInvitationGuide);
  }

  async function loadCheckInSettings() {
    setError('');
    try {
      applyCheckInSettings(await fetchCheckInSettings());
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '加载签到设置失败');
    }
  }

  async function loadFavoriteSites(nextPage = favoritePage) {
    setLoading(true);
    setError('');
    try {
      const [pageData, groupsData] = await Promise.all([
        fetchFavoriteSites({ keyword: favoriteKeyword, group: favoriteGroup, page: nextPage, size: 10 }),
        fetchFavoriteSiteGroups()
      ]);
      setFavoriteSites(pageData.content);
      favoriteSitesRef.current = pageData.content;
      setFavoriteGroups(groupsData);
      setFavoritePage(pageData.number);
      setFavoriteTotalPages(Math.max(pageData.totalPages, 1));
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '加载收藏网站失败');
    } finally {
      setLoading(false);
    }
  }

  async function loadRechargeActivities(
    nextClaimPage = rechargeClaimPage,
    claimFilters = { keyword: rechargeClaimKeyword, status: rechargeClaimStatus }
  ) {
    setLoading(true);
    setError('');
    try {
      const [activities, claims, rewardStats] = await Promise.all([
        fetchRechargeActivities(),
        fetchRechargeRewardClaims({
          keyword: claimFilters.keyword,
          status: claimFilters.status,
          page: nextClaimPage,
          size: 10
        }),
        fetchRechargeRewardStats()
      ]);
      setRechargeActivities(activities);
      setRechargeClaims(claims.content);
      setRechargeClaimPage(claims.number);
      setRechargeClaimTotalPages(Math.max(claims.totalPages, 1));
      setRechargeRewardStats(rewardStats);
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '加载充值活动失败');
    } finally {
      setLoading(false);
    }
  }

  async function loadCheckInStats() {
    setLoading(true);
    setError('');
    try {
      setCheckInStats(await fetchCheckInStats());
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '加载签到统计失败');
    } finally {
      setLoading(false);
    }
  }

  async function loadGroupRateMonitor() {
    setLoading(true);
    setError('');
    try {
      applyGroupRateMonitor(await fetchSub2APIGroupRateMonitor());
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '加载倍率监控失败');
    } finally {
      setLoading(false);
    }
  }

  async function loadInvitationRecords(
    nextPage = invitationRecordPage,
    filters = { keyword: invitationRecordKeyword, status: invitationRecordStatus }
  ) {
    setLoading(true);
    setError('');
    try {
      const [result, stats] = await Promise.all([
        fetchInvitationRecords({
          keyword: filters.keyword,
          status: filters.status,
          page: nextPage,
          size: 20
        }),
        fetchInvitationStats()
      ]);
      setInvitationRecords(result.content);
      setInvitationRecordPage(result.number);
      setInvitationRecordTotalPages(Math.max(result.totalPages, 1));
      setInvitationRecordTotal(result.totalElements);
      setInvitationStats(stats);
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '加载邀请记录失败');
    } finally {
      setLoading(false);
    }
  }

  async function checkSystemUpdate(showNotice = false) {
    setSystemUpdateChecking(true);
    setError('');
    try {
      const result = await fetchSystemUpdateCheck();
      setSystemUpdate(result);
      if (showNotice) {
        notifySuccess(result.message || '版本检测完成');
      }
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '检测版本更新失败');
    } finally {
      setSystemUpdateChecking(false);
    }
  }

  async function updateSystemVersion() {
    if (!systemUpdate?.updateEnabled) {
      notifyError('当前部署未启用后台更新，请检查 SYSTEM_UPDATE_ENABLED 和内置更新脚本');
      return;
    }
    if (!await confirmDialog({
      title: '执行版本更新',
      message: '确认在后端容器中执行系统更新命令？更新过程中服务可能短暂不可用。',
      confirmText: '执行更新'
    })) {
      return;
    }
    setSystemUpdating(true);
    setSystemUpdateOutput('');
    try {
      const result = await runSystemUpdate();
      setSystemUpdateOutput(result.output || result.message);
      notifySuccess(result.message || '更新命令已执行');
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '执行更新失败');
    } finally {
      setSystemUpdating(false);
    }
  }

  function applyGroupRateMonitor(data: Sub2APIGroupRateMonitor) {
    setGroupRateMonitor(data);
    setGroupRateDraft(data.settings);
    setGroupRateLogDrafts(Object.fromEntries(
      (data.logs ?? []).map((entry) => [entry.id, {
        oldRate: Number(entry.oldRate).toFixed(6),
        newRate: Number(entry.newRate).toFixed(6),
        createdAt: toDateTimeLocal(entry.createdAt),
        publicVisible: entry.publicVisible
      }])
    ));
  }

  useEffect(() => {
    load(0);
  }, []);

  useEffect(() => {
    if (activeSection === 'favorites') {
      loadFavoriteSites(0);
    }
    if (activeSection === 'recharge') {
      loadRechargeActivities();
    }
    if (activeSection === 'invitations') {
      loadInvitationRecords(0);
    }
    if (activeSection === 'checkins') {
      load(0);
      loadCheckInStats();
    }
    if (activeSection === 'rates') {
      loadGroupRateMonitor();
    }
    if (activeSection === 'system') {
      checkSystemUpdate(false);
    }
  }, [activeSection]);

  const amountOptions = useMemo(() => toAmountOptions(stats.amountStats, [...prizeTierDrafts, ...socialPrizeTierDrafts]), [stats.amountStats, prizeTierDrafts, socialPrizeTierDrafts]);
  const navItems = [
    { key: 'recharge' as const, label: '充值活动', icon: CircleDollarSign },
    { key: 'invitations' as const, label: '邀请记录', icon: UserPlus },
    { key: 'checkins' as const, label: '签到管理', icon: CalendarCheck2 },
    { key: 'favorites' as const, label: '网站收藏', icon: Bookmark },
    { key: 'rates' as const, label: '倍率监控', icon: Globe2 },
    { key: 'system' as const, label: '系统设置', icon: Settings2 }
  ];
  const rechargeClaimStatusOptions = [
    { value: '', label: '全部状态' },
    { value: 'CLAIMED', label: '已领取' },
    { value: 'PENDING', label: '处理中' },
    { value: 'FAILED', label: '失败' }
  ];
  const invitationRecordStatusOptions = [
    { value: '', label: '全部状态' },
    { value: 'REWARDED', label: '已奖励' },
    { value: 'PENDING', label: '处理中' },
    { value: 'FAILED', label: '失败' }
  ];

  function rechargeClaimStatusText(value: string) {
    return rechargeClaimStatusOptions.find((item) => item.value === value)?.label ?? value;
  }

  function invitationRecordStatusText(value: string) {
    return invitationRecordStatusOptions.find((item) => item.value === value)?.label ?? value;
  }

  const hasSettingsChanges = settingsChanged(
    dailyMaxUsers,
    dailyMaxUsersDraft,
    dailyLimitMode,
    dailyLimitModeDraft,
    directDailyMaxUsers,
    directDailyMaxUsersDraft,
    socialDailyMaxUsers,
    socialDailyMaxUsersDraft,
    prizeTiers,
    prizeTierDrafts,
    socialPrizeTiers,
    socialPrizeTierDrafts,
    groupLink,
    groupLinkDraft,
    frontendPublicUrl,
    frontendPublicUrlDraft,
    adminSettings,
    adminSettingsDraft,
    sub2api,
    sub2apiDraft,
    invitationSettings,
    invitationSettingsDraft,
    invitationGuide,
    invitationGuideDraft,
    telegram,
    telegramDraft
  );

  function logout() {
    clearToken();
    onLogout();
  }

  async function remove(id: number) {
    if (!await confirmDialog({
      title: '删除兑换码',
      message: '确认删除这条兑换码记录？',
      confirmText: '删除',
      danger: true
    })) {
      return;
    }
    await deleteCode(id);
    load(page);
  }

  async function saveCheckInSettings(event: FormEvent) {
    event.preventDefault();
    const nextDailyMaxUsers = Number(dailyMaxUsersDraft);
    if (!Number.isInteger(nextDailyMaxUsers) || nextDailyMaxUsers < 0) {
      notifyError('每日签到上限必须是大于等于 0 的整数');
      return;
    }
    const nextDirectDailyMaxUsers = Number(directDailyMaxUsersDraft);
    const nextSocialDailyMaxUsers = Number(socialDailyMaxUsersDraft);
    if (dailyLimitModeDraft === 'separate' && (!Number.isInteger(nextDirectDailyMaxUsers) || nextDirectDailyMaxUsers < 0)) {
      notifyError('站内每日上限必须是大于等于 0 的整数');
      return;
    }
    if (dailyLimitModeDraft === 'separate' && (!Number.isInteger(nextSocialDailyMaxUsers) || nextSocialDailyMaxUsers < 0)) {
      notifyError('社交每日上限必须是大于等于 0 的整数');
      return;
    }

    const parsedDirectPrizeTiers = parsePrizeTierDrafts(prizeTierDrafts);
    if (typeof parsedDirectPrizeTiers === 'string') {
      notifyError(`站内签到：${parsedDirectPrizeTiers}`);
      return;
    }
    const parsedSocialPrizeTiers = parsePrizeTierDrafts(socialPrizeTierDrafts);
    if (typeof parsedSocialPrizeTiers === 'string') {
      notifyError(`社交签到：${parsedSocialPrizeTiers}`);
      return;
    }
    const parsedAdminSettings = {
      ...adminSettingsDraft,
      username: adminSettingsDraft.username.trim(),
      password: adminSettingsDraft.password ?? ''
    };
    if (!parsedAdminSettings.username) {
      notifyError('后台管理员账号不能为空');
      return;
    }
    const invitationAmount = Number(invitationSettingsDraft.amount);
    if (!Number.isFinite(invitationAmount) || invitationAmount < 0) {
      notifyError('邀请奖励金额必须是大于等于 0 的数字');
      return;
    }
    if ((invitationSettingsDraft.afterTime && invitationAmount <= 0) || (!invitationSettingsDraft.afterTime && invitationAmount > 0)) {
      notifyError('邀请活动的新人时间门槛和奖励金额必须同时设置');
      return;
    }
    const parsedInvitation: InvitationSettings = {
      afterTime: invitationSettingsDraft.afterTime ? new Date(invitationSettingsDraft.afterTime).toISOString() : '',
      amount: invitationAmount
    };
    const parsedInvitationGuide: InvitationGuideSettings = {
      qqGroupNumber: invitationGuideDraft.qqGroupNumber.trim(),
      qqBotMention: invitationGuideDraft.qqBotMention.trim()
    };
    if (!parsedInvitationGuide.qqGroupNumber || !parsedInvitationGuide.qqBotMention) {
      notifyError('QQ 邀请教程的群号和机器人称呼不能为空');
      return;
    }

    setSettingsSaving(true);
    setSettingsSaved(false);
    setError('');
    try {
      const parsedSub2API = parseSub2APIDraft(sub2apiDraft);
      if (typeof parsedSub2API === 'string') {
        notifyError(parsedSub2API);
        return;
      }
      const parsedTelegram = parseTelegramDraft(telegramDraft);
      if (typeof parsedTelegram === 'string') {
        notifyError(parsedTelegram);
        return;
      }

      const settings = await updateCheckInSettings(nextDailyMaxUsers, dailyLimitModeDraft, nextDirectDailyMaxUsers, nextSocialDailyMaxUsers, parsedDirectPrizeTiers, parsedSocialPrizeTiers, groupLinkDraft.trim(), frontendPublicUrlDraft.trim().replace(/\/+$/, ''), parsedAdminSettings, parsedSub2API, parsedInvitation, parsedInvitationGuide, parsedTelegram);
      setDailyMaxUsers(settings.dailyMaxUsers);
      setDailyMaxUsersDraft(String(settings.dailyMaxUsers));
      const nextLimitMode = settings.dailyLimitMode === 'separate' ? 'separate' : 'shared';
      const savedDirectDailyMaxUsers = Number.isFinite(Number(settings.directDailyMaxUsers)) ? Number(settings.directDailyMaxUsers) : settings.dailyMaxUsers;
      const savedSocialDailyMaxUsers = Number.isFinite(Number(settings.socialDailyMaxUsers)) ? Number(settings.socialDailyMaxUsers) : settings.dailyMaxUsers;
      setDailyLimitMode(nextLimitMode);
      setDailyLimitModeDraft(nextLimitMode);
      setDirectDailyMaxUsers(savedDirectDailyMaxUsers);
      setDirectDailyMaxUsersDraft(String(savedDirectDailyMaxUsers));
      setSocialDailyMaxUsers(savedSocialDailyMaxUsers);
      setSocialDailyMaxUsersDraft(String(savedSocialDailyMaxUsers));
      const directTiers = Array.isArray(settings.directPrizeTiers) ? settings.directPrizeTiers : settings.prizeTiers;
      const socialTiers = Array.isArray(settings.socialPrizeTiers) ? settings.socialPrizeTiers : directTiers;
      setPrizeTiers(directTiers);
      setPrizeTierDrafts(toPrizeTierDrafts(directTiers));
      setSocialPrizeTiers(socialTiers);
      setSocialPrizeTierDrafts(toPrizeTierDrafts(socialTiers));
      setGroupLink(settings.groupLink ?? '');
      setGroupLinkDraft(settings.groupLink ?? '');
      setFrontendPublicUrl(settings.frontendPublicUrl ?? '');
      setFrontendPublicUrlDraft(settings.frontendPublicUrl ?? '');
      const savedAdmin = settings.admin ?? emptyAdminSettings;
      setAdminSettings(savedAdmin);
      setAdminSettingsDraft({ ...savedAdmin, password: '' });
      setSub2api(settings.sub2api);
      setSub2apiDraft(toSub2APIDraft(settings.sub2api));
      const savedTelegram = settings.telegram ?? emptyTelegramSettings;
      setTelegram(savedTelegram);
      setTelegramDraft(toTelegramDraft(savedTelegram));
      const savedInvitation = settings.invitation ?? emptyInvitationSettings;
      const savedInvitationDraft = { ...savedInvitation, afterTime: toDateTimeLocal(savedInvitation.afterTime) };
      setInvitationSettings(savedInvitationDraft);
      setInvitationSettingsDraft(savedInvitationDraft);
      const savedInvitationGuide = settings.invitationGuide ?? emptyInvitationGuideSettings;
      setInvitationGuide(savedInvitationGuide);
      setInvitationGuideDraft(savedInvitationGuide);
      setSettingsSaved(true);
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '保存设置失败');
    } finally {
      setSettingsSaving(false);
    }
  }

  async function connectTelegram() {
    if (hasSettingsChanges) {
      notifyError('请先保存 Telegram 配置，再测试连接');
      return;
    }
    setTelegramConnecting(true);
    try {
      const result = await connectTelegramBot();
      const nextTelegram = { ...telegram, ...result };
      setTelegram(nextTelegram);
      setTelegramDraft(toTelegramDraft(nextTelegram));
      notifySuccess(result.botUsername ? `Telegram 已连接：@${result.botUsername}` : 'Telegram 已连接');
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '连接 Telegram 失败');
    } finally {
      setTelegramConnecting(false);
    }
  }

  async function saveGroupRateMonitor(event: FormEvent) {
    event.preventDefault();
    setGroupRateSaving(true);
    setGroupRateSaved(false);
    setError('');
    try {
      applyGroupRateMonitor(await updateSub2APIGroupRateMonitor(groupRateDraft));
      setGroupRateSaved(true);
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '保存倍率监控失败');
    } finally {
      setGroupRateSaving(false);
    }
  }

  async function refreshGroupRatesNow() {
    setGroupRateRefreshing(true);
    setError('');
    try {
      applyGroupRateMonitor(await refreshSub2APIGroupRates());
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '刷新分组倍率失败');
    } finally {
      setGroupRateRefreshing(false);
    }
  }

  async function saveGroupRateLog(id: number) {
    const draft = groupRateLogDrafts[id];
    if (!draft) return;
    const oldRate = Number(draft.oldRate);
    const newRate = Number(draft.newRate);
    if (!Number.isFinite(oldRate) || oldRate <= 0 || !Number.isFinite(newRate) || newRate <= 0) {
      notifyError('旧倍率和新倍率都必须是大于 0 的数字');
      return;
    }
    setGroupRateEditingKey(`log:${id}`);
    setError('');
    try {
      applyGroupRateMonitor(await updateSub2APIGroupRateLog(id, {
        oldRate,
        newRate,
        createdAt: draft.createdAt,
        publicVisible: draft.publicVisible
      }));
      notifySuccess('倍率日志已更新');
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '保存倍率日志失败');
    } finally {
      setGroupRateEditingKey(null);
    }
  }

  function patchGroupRateDraft(patch: Partial<Sub2APIGroupRateMonitorSettings>) {
    setGroupRateDraft((current) => ({ ...current, ...patch }));
    setGroupRateSaved(false);
  }

  function toggleGroupRateId(key: 'monitoredGroupIds' | 'publicGroupIds', groupId: string, checked: boolean) {
    setGroupRateDraft((current) => {
      const values = new Set(current[key]);
      if (checked) {
        values.add(groupId);
      } else {
        values.delete(groupId);
      }
      return { ...current, [key]: Array.from(values).sort() };
    });
    setGroupRateSaved(false);
  }

  function patchGroupRateLogDraft(id: number, patch: Partial<{ oldRate: string; newRate: string; createdAt: string; publicVisible: boolean }>) {
    setGroupRateLogDrafts((current) => ({
      ...current,
      [id]: { ...current[id], ...patch }
    }));
  }

  function setFavoriteCardRef(id: number, element: HTMLElement | null) {
    if (element) {
      favoriteCardRefs.current.set(id, element);
    } else {
      favoriteCardRefs.current.delete(id);
    }
  }

  function animateFavoriteReorder(nextSites: FavoriteSite[]) {
    const previousRects = new Map<number, DOMRect>();
    favoriteCardRefs.current.forEach((element, id) => {
      previousRects.set(id, element.getBoundingClientRect());
    });

    favoriteSitesRef.current = nextSites;
    setFavoriteSites(nextSites);

    window.requestAnimationFrame(() => {
      nextSites.forEach((site) => {
        const element = favoriteCardRefs.current.get(site.id);
        const previousRect = previousRects.get(site.id);
        if (!element || !previousRect) return;
        const nextRect = element.getBoundingClientRect();
        const deltaX = previousRect.left - nextRect.left;
        const deltaY = previousRect.top - nextRect.top;
        if (deltaX === 0 && deltaY === 0) return;

        element.style.transition = 'none';
        element.style.transform = `translate(${deltaX}px, ${deltaY}px)`;
        element.style.zIndex = site.id === draggedFavoriteId ? '2' : '1';
        window.requestAnimationFrame(() => {
          element.style.transition = 'transform 180ms cubic-bezier(0.2, 0.8, 0.2, 1)';
          element.style.transform = '';
          window.setTimeout(() => {
            element.style.transition = '';
            element.style.zIndex = '';
          }, 210);
        });
      });
    });
  }

  function moveFavoriteSiteInView(targetId: number) {
    if (draggedFavoriteId === null || draggedFavoriteId === targetId) {
      return;
    }
    const currentSites = favoriteSitesRef.current;
    const draggedIndex = currentSites.findIndex((site) => site.id === draggedFavoriteId);
    const targetIndex = currentSites.findIndex((site) => site.id === targetId);
    if (draggedIndex < 0 || targetIndex < 0) {
      return;
    }

    const nextSites = [...currentSites];
    const [draggedSite] = nextSites.splice(draggedIndex, 1);
    nextSites.splice(targetIndex, 0, draggedSite);
    favoriteOrderChangedRef.current = true;
    animateFavoriteReorder(nextSites);
  }

  async function finishFavoriteDrag() {
    const shouldSave = favoriteOrderChangedRef.current;
    favoriteOrderChangedRef.current = false;
    setDraggedFavoriteId(null);
    if (!shouldSave) {
      return;
    }

    const currentSites = favoriteSitesRef.current;
    const baseSort = favoritePage * 10;
    const sortedSites = currentSites.map((site, index) => ({ ...site, sort: baseSort + index }));

    favoriteSitesRef.current = sortedSites;
    setFavoriteSites(sortedSites);
    setError('');
    try {
      await Promise.all(sortedSites.map((site) => updateFavoriteSite(site.id, {
        icon: site.icon,
        url: site.url,
        name: site.name,
        description: site.description,
        group: site.group,
        sort: site.sort
      })));
      loadFavoriteSites(favoritePage);
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '保存排序失败');
      loadFavoriteSites(favoritePage);
    }
  }

  return (
    <main className="admin-layout">
      <aside className="sidebar">
        <div className="sidebar-brand">
          <div className="brand-mark compact">
            <ShieldCheck size={21} />
          </div>
          <div>
            <strong>兑换码管理</strong>
            <span>Daily Check-in</span>
          </div>
        </div>
        <nav className="side-nav">
          {navItems.map((item) => {
            const Icon = item.icon;
            return (
              <button
                key={item.key}
                className={activeSection === item.key ? 'is-active' : ''}
                onClick={() => onSectionChange(item.key)}
                type="button"
              >
                <Icon size={18} />
                {item.label}
              </button>
            );
          })}
        </nav>
      </aside>

      <section className="app-shell">
      <header className="topbar">
        <div>
          <span className="eyebrow">Daily Check-in Reward</span>
          <h1>{sectionTitle(activeSection)}</h1>
        </div>
        <button className="ghost-btn" onClick={logout} title="退出登录">
          <LogOut size={18} />
          退出
        </button>
      </header>

      {error && <div className="error-banner">{error}</div>}

      {activeSection === 'checkins' && (
        <>
      <section className="summary-grid checkin-summary-grid">
        <article className="metric metric-green">
          <span>今日签到消耗</span>
          <strong>{Number(checkInStats.todayAmount).toFixed(2)}</strong>
        </article>
        <article className="metric metric-blue">
          <span>今日签到人数</span>
          <strong>{checkInStats.todayUsers}</strong>
        </article>
      </section>

      <section className="checkin-chart-grid">
        <CheckInTrendChart
          title="30 日签到消耗金额"
          daily={checkInStats.daily}
          valueKey="amount"
          unit="元"
          color="#16a34a"
        />
        <CheckInTrendChart
          title="30 日签到人数"
          daily={checkInStats.daily}
          valueKey="users"
          unit="人"
          color="#2563eb"
        />
      </section>

      <form className="settings-panel checkin-settings" onSubmit={saveCheckInSettings}>
        <div className="settings-panel-head checkin-settings-head">
          <div className="settings-title">
            <Settings2 size={18} />
            <span>签到设置</span>
          </div>
          <div className="checkin-actions">
            <label className="daily-limit-field daily-limit-mode-field">
              上限模式
              <select
                value={dailyLimitModeDraft}
                onChange={(event) => {
                  const nextMode = event.target.value === 'separate' ? 'separate' : 'shared';
                  setDailyLimitModeDraft(nextMode);
                  setSettingsSaved(false);
                }}
              >
                <option value="shared">共享上限</option>
                <option value="separate">分开上限</option>
              </select>
            </label>
            {dailyLimitModeDraft === 'shared' && (
              <label className="daily-limit-field">
                每日上限
                <input
                  type="number"
                  min="0"
                  step="1"
                  value={dailyMaxUsersDraft}
                  onChange={(event) => {
                    setDailyMaxUsersDraft(event.target.value);
                    setSettingsSaved(false);
                  }}
                />
              </label>
            )}
            {dailyLimitModeDraft === 'separate' && (
              <>
                <label className="daily-limit-field">
                  站内上限
                  <input
                    type="number"
                    min="0"
                    step="1"
                    value={directDailyMaxUsersDraft}
                    onChange={(event) => {
                      setDirectDailyMaxUsersDraft(event.target.value);
                      setSettingsSaved(false);
                    }}
                  />
                </label>
                <label className="daily-limit-field">
                  社交上限
                  <input
                    type="number"
                    min="0"
                    step="1"
                    value={socialDailyMaxUsersDraft}
                    onChange={(event) => {
                      setSocialDailyMaxUsersDraft(event.target.value);
                      setSettingsSaved(false);
                    }}
                  />
                </label>
              </>
            )}
            <button className="ghost-btn" type="submit" disabled={settingsSaving || !hasSettingsChanges}>
              <CheckCircle2 size={17} />
              {settingsSaving ? '保存中...' : '保存'}
            </button>
            {settingsSaved && <span className="settings-saved">已保存</span>}
          </div>
        </div>

        <label className="checkin-link-field">
          群链接
          <input
            value={groupLinkDraft}
            onChange={(event) => {
              setGroupLinkDraft(event.target.value);
              setSettingsSaved(false);
            }}
            placeholder="https://..."
          />
        </label>

        <div className="invitation-settings-editor">
          <div className="settings-title">
            <UserPlus size={18} />
            <span>邀请奖励</span>
          </div>
          <div className="invitation-settings-grid">
            <label>
              新人账号创建时间必须晚于
              <input
                type="datetime-local"
                value={invitationSettingsDraft.afterTime}
                onChange={(event) => {
                  setInvitationSettingsDraft((current) => ({ ...current, afterTime: event.target.value }));
                  setSettingsSaved(false);
                }}
              />
            </label>
            <label>
              每次成功邀请奖励余额
              <input
                type="number"
                min="0"
                step="0.01"
                value={invitationSettingsDraft.amount}
                onChange={(event) => {
                  setInvitationSettingsDraft((current) => ({ ...current, amount: Number(event.target.value) }));
                  setSettingsSaved(false);
                }}
              />
            </label>
          </div>
          <p>只有通过群机器人链接完成社交账号绑定，且 Sub2API 账号创建时间严格晚于该门槛，邀请人才会获得奖励。</p>
        </div>

        <div className="invitation-settings-editor">
          <div className="settings-title">
            <UserPlus size={18} />
            <span>QQ 邀请教程</span>
          </div>
          <div className="invitation-settings-grid">
            <label>
              QQ 群号
              <input
                value={invitationGuideDraft.qqGroupNumber}
                maxLength={100}
                onChange={(event) => {
                  setInvitationGuideDraft((current) => ({ ...current, qqGroupNumber: event.target.value }));
                  setSettingsSaved(false);
                }}
                placeholder="799128896"
              />
            </label>
            <label>
              QQ 机器人称呼
              <input
                value={invitationGuideDraft.qqBotMention}
                maxLength={100}
                onChange={(event) => {
                  setInvitationGuideDraft((current) => ({ ...current, qqBotMention: event.target.value }));
                  setSettingsSaved(false);
                }}
                placeholder="@咕咕嘎嘎"
              />
            </label>
          </div>
          <p>用户邀请教程会组合显示“机器人称呼 + 绑定 + 邀请码”。群链接继续使用上方配置。</p>
        </div>

        <div className="checkin-tier-grid">
        <div className="tier-editor">
          <div className="tier-editor-head">
            <span>站内签到金额概率</span>
            <div className={`probability-total ${prizeTierTotal(prizeTierDrafts) === 100 ? 'is-valid' : ''}`}>
              合计 {prizeTierTotal(prizeTierDrafts).toFixed(2)}%
            </div>
            <button
              type="button"
              className="ghost-btn compact-action"
              onClick={() => {
                setPrizeTierDrafts((current) => [...current, { amount: amountOptions[0] ?? '1.00', probability: '1.00' }]);
                setSettingsSaved(false);
              }}
            >
              <Plus size={16} />
              添加站内档位
            </button>
          </div>
          <div className="tier-table">
            <datalist id="prize-amount-options">
              {amountOptions.map((amount) => (
                <option key={amount} value={amount} />
              ))}
            </datalist>
            <div className="tier-table-head">
              <span>金额</span>
              <span>概率 %</span>
              <span>操作</span>
            </div>
            <div className="tier-list">
              {prizeTierDrafts.map((tier, index) => (
                <div className="tier-row" key={index}>
                  <input
                    aria-label={`第 ${index + 1} 档金额`}
                    list="prize-amount-options"
                    type="number"
                    min="0.01"
                    step="0.01"
                    value={tier.amount}
                    onChange={(event) => updatePrizeTierDraft(index, 'amount', event.target.value, setPrizeTierDrafts, setSettingsSaved)}
                  />
                  <input
                    aria-label={`第 ${index + 1} 档概率`}
                    type="number"
                    min="0.01"
                    max="100"
                    step="0.01"
                    value={tier.probability}
                    onChange={(event) => updatePrizeTierDraft(index, 'probability', event.target.value, setPrizeTierDrafts, setSettingsSaved)}
                  />
                  <button
                    type="button"
                    className="icon-btn"
                    title="删除"
                    disabled={prizeTierDrafts.length <= 1}
                    onClick={() => {
                      setPrizeTierDrafts((current) => current.filter((_, currentIndex) => currentIndex !== index));
                      setSettingsSaved(false);
                    }}
                  >
                    <Trash2 size={16} />
                  </button>
                </div>
              ))}
            </div>
          </div>
        </div>

        <div className="tier-editor">
          <div className="tier-editor-head">
            <span>社交签到金额概率</span>
            <div className={`probability-total ${prizeTierTotal(socialPrizeTierDrafts) === 100 ? 'is-valid' : ''}`}>
              合计 {prizeTierTotal(socialPrizeTierDrafts).toFixed(2)}%
            </div>
            <button
              type="button"
              className="ghost-btn compact-action"
              onClick={() => {
                setSocialPrizeTierDrafts((current) => [...current, { amount: amountOptions[0] ?? '1.00', probability: '1.00' }]);
                setSettingsSaved(false);
              }}
            >
              <Plus size={16} />
              添加社交档位
            </button>
          </div>
          <div className="tier-table">
            <div className="tier-table-head">
              <span>金额</span>
              <span>概率 %</span>
              <span>操作</span>
            </div>
            <div className="tier-list">
              {socialPrizeTierDrafts.map((tier, index) => (
                <div className="tier-row" key={index}>
                  <input
                    aria-label={`社交第 ${index + 1} 档金额`}
                    list="prize-amount-options"
                    type="number"
                    min="0.01"
                    step="0.01"
                    value={tier.amount}
                    onChange={(event) => updatePrizeTierDraft(index, 'amount', event.target.value, setSocialPrizeTierDrafts, setSettingsSaved)}
                  />
                  <input
                    aria-label={`社交第 ${index + 1} 档概率`}
                    type="number"
                    min="0.01"
                    max="100"
                    step="0.01"
                    value={tier.probability}
                    onChange={(event) => updatePrizeTierDraft(index, 'probability', event.target.value, setSocialPrizeTierDrafts, setSettingsSaved)}
                  />
                  <button
                    type="button"
                    className="icon-btn"
                    title="删除"
                    disabled={socialPrizeTierDrafts.length <= 1}
                    onClick={() => {
                      setSocialPrizeTierDrafts((current) => current.filter((_, currentIndex) => currentIndex !== index));
                      setSettingsSaved(false);
                    }}
                  >
                    <Trash2 size={16} />
                  </button>
                </div>
              ))}
            </div>
          </div>
        </div>
        </div>

      </form>

      <section className="toolbar">
        <div className="search-box">
          <Search size={18} />
          <input
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
            onKeyDown={(event) => event.key === 'Enter' && load(0)}
            placeholder="搜索兑换码或用户 ID"
          />
        </div>
        <select value={status} onChange={(event) => setStatus(event.target.value)}>
          <option value="">全部状态</option>
          <option value="AVAILABLE">未绑定</option>
          <option value="ASSIGNED">已绑定</option>
          <option value="USED">已使用</option>
          <option value="VOIDED">已作废</option>
        </select>
        <button className="ghost-btn" onClick={() => load(0)}>
          <Search size={17} />
          查询
        </button>
        <button className="ghost-btn" type="button" onClick={loadCheckInStats}>
          <CheckCircle2 size={17} />
          刷新统计
        </button>
        <button
          className="primary-btn"
          onClick={() => {
            setImportOpen(true);
          }}
        >
          <Plus size={18} />
          批量导入
        </button>
      </section>

      <section className="table-panel">
        <table>
          <thead>
            <tr>
              <th>兑换码</th>
              <th>用户 ID</th>
              <th>日期</th>
              <th>金额</th>
              <th>状态</th>
              <th>创建时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {codes.map((code) => (
              <tr key={code.id}>
                <td className="code-cell">{code.code}</td>
                <td>{code.userId || '-'}</td>
                <td>{code.signDate || '-'}</td>
                <td className="amount-cell">
                  <CircleDollarSign size={16} />
                  {Number(code.amount).toFixed(2)}
                </td>
                <td><span className={`status status-${code.status.toLowerCase()}`}>{statusText[code.status]}</span></td>
                <td>{formatDateTime(code.createdAt)}</td>
                <td>
                  <div className="row-actions">
                    <button title="编辑" onClick={() => { setEditing(code); setModalOpen(true); }}>
                      <Pencil size={16} />
                    </button>
                    <button title="删除" onClick={() => remove(code.id)}>
                      <Trash2 size={16} />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            {!loading && codes.length === 0 && (
              <tr>
                <td colSpan={7} className="empty-cell">暂无兑换码记录</td>
              </tr>
            )}
            {loading && (
              <tr>
                <td colSpan={7} className="empty-cell">加载中...</td>
              </tr>
            )}
          </tbody>
        </table>
      </section>

      <footer className="pager">
        <button disabled={page <= 0} onClick={() => load(page - 1)}>上一页</button>
        <span>{page + 1} / {totalPages}</span>
        <button disabled={page + 1 >= totalPages} onClick={() => load(page + 1)}>下一页</button>
      </footer>
        </>
      )}

      {activeSection === 'favorites' && (
        <>
      <section className="toolbar favorite-toolbar">
        <div className="search-box">
          <Search size={18} />
          <input
            value={favoriteKeyword}
            onChange={(event) => setFavoriteKeyword(event.target.value)}
            onKeyDown={(event) => event.key === 'Enter' && loadFavoriteSites(0)}
            placeholder="搜索名称、URL、简介或分组"
          />
        </div>
        <select
          value={favoriteGroup}
          onChange={(event) => setFavoriteGroup(event.target.value)}
        >
          <option value="">全部分组</option>
          {favoriteGroups.map((group) => (
            <option key={group} value={group}>{group}</option>
          ))}
        </select>
        <button className="ghost-btn" onClick={() => loadFavoriteSites(0)}>
          <Search size={17} />
          查询
        </button>
        <button
          className="primary-btn"
          onClick={() => {
            setEditingFavorite(null);
            setFavoriteModalOpen(true);
          }}
        >
          <Plus size={18} />
          新增网站
        </button>
      </section>

      <section className="favorite-card-panel">
        <div className="favorite-card-grid">
          {favoriteSites.map((site) => (
            <article
              className={`favorite-card ${draggedFavoriteId === site.id ? 'is-dragging' : ''}`}
              key={site.id}
              ref={(element) => setFavoriteCardRef(site.id, element)}
              onDragOver={(event) => {
                if (draggedFavoriteId === null) return;
                event.preventDefault();
                moveFavoriteSiteInView(site.id);
              }}
              onDrop={(event) => {
                event.preventDefault();
                finishFavoriteDrag();
              }}
              onDragEnd={() => finishFavoriteDrag()}
            >
              <a className="favorite-card-link" href={site.url} target="_blank" rel="noreferrer" aria-label={`打开 ${site.name}`}>
                <div className="favorite-card-main">
                  <SiteIcon site={site} />
                  <div className="favorite-card-content">
                    <div className="favorite-card-title">
                    <span>{site.name}</span>
                    <ExternalLink size={15} />
                    </div>
                    <p>{site.description || '暂无简介'}</p>
                  </div>
                </div>
              </a>
              <div className="favorite-card-footer">
                <span className="favorite-group-pill">{site.group || '未分组'}</span>
                <div className="favorite-card-actions">
                  <button className="icon-btn" title="编辑" onClick={() => { setEditingFavorite(site); setFavoriteModalOpen(true); }}>
                    <Pencil size={16} />
                  </button>
                  <button
                    className="icon-btn drag-handle"
                    title="拖动排序"
                    draggable
                    onDragStart={(event) => {
                      setDraggedFavoriteId(site.id);
                      favoriteOrderChangedRef.current = false;
                      event.dataTransfer.effectAllowed = 'move';
                      event.dataTransfer.setData('text/plain', String(site.id));
                    }}
                    type="button"
                  >
                    <GripVertical size={16} />
                  </button>
                </div>
              </div>
            </article>
          ))}
        </div>
        {!loading && favoriteSites.length === 0 && (
          <div className="empty-cell">暂无收藏网站</div>
        )}
        {loading && (
          <div className="empty-cell">加载中...</div>
        )}
      </section>

      <footer className="pager">
        <button disabled={favoritePage <= 0} onClick={() => loadFavoriteSites(favoritePage - 1)}>上一页</button>
        <span>{favoritePage + 1} / {favoriteTotalPages}</span>
        <button disabled={favoritePage + 1 >= favoriteTotalPages} onClick={() => loadFavoriteSites(favoritePage + 1)}>下一页</button>
      </footer>
        </>
      )}

      {activeSection === 'invitations' && (
        <>
          <InvitationTrendChart daily={invitationStats?.daily ?? []} />
          <section className="table-panel invitation-record-panel">
          <form
            className="toolbar invitation-record-toolbar"
            onSubmit={(event) => {
              event.preventDefault();
              loadInvitationRecords(0);
            }}
          >
            <div className="settings-title invitation-record-title">
              <UserPlus size={18} />
              <span>邀请记录</span>
              <strong>{invitationRecordTotal}</strong>
            </div>
            <div className="search-box">
              <Search size={17} />
              <input
                value={invitationRecordKeyword}
                onChange={(event) => setInvitationRecordKeyword(event.target.value)}
                placeholder="邀请码、用户 ID、平台账号"
              />
            </div>
            <select value={invitationRecordStatus} onChange={(event) => setInvitationRecordStatus(event.target.value)}>
              {invitationRecordStatusOptions.map((item) => (
                <option key={item.value || 'all'} value={item.value}>{item.label}</option>
              ))}
            </select>
            <button className="ghost-btn" type="submit" disabled={loading}>
              <Search size={17} />
              查询
            </button>
            <button
              className="ghost-btn"
              type="button"
              disabled={loading}
              onClick={() => {
                setInvitationRecordKeyword('');
                setInvitationRecordStatus('');
                loadInvitationRecords(0, { keyword: '', status: '' });
              }}
            >
              <CheckCircle2 size={17} />
              刷新
            </button>
          </form>

          <div className="invitation-table-scroll">
            <table>
              <thead>
                <tr>
                  <th>邀请码</th>
                  <th>邀请人</th>
                  <th>新用户</th>
                  <th>平台账号</th>
                  <th>新人注册时间</th>
                  <th>奖励</th>
                  <th>状态</th>
                  <th>绑定 / 发放时间</th>
                </tr>
              </thead>
              <tbody>
                {invitationRecords.map((record) => (
                  <tr key={record.id}>
                    <td><code>{record.inviteCode}</code></td>
                    <td><strong>UID {record.inviterUserId}</strong></td>
                    <td><strong>UID {record.inviteeUserId}</strong></td>
                    <td>
                      <strong>{formatPlatformName(record.platform)}</strong>
                      <small>{record.externalUserId}</small>
                    </td>
                    <td>{formatDateTime(record.inviteeCreatedAt)}</td>
                    <td>{Number(record.rewardAmount).toFixed(2)}</td>
                    <td>
                      <span className={`claim-status claim-${record.rewardStatus === 'REWARDED' ? 'claimed' : record.rewardStatus.toLowerCase()}`}>
                        {invitationRecordStatusText(record.rewardStatus)}
                      </span>
                      {record.errorMessage && <small className="claim-error invitation-error">{record.errorMessage}</small>}
                    </td>
                    <td>
                      <strong>{formatDateTime(record.createdAt)}</strong>
                      <small>{record.rewardedAt ? `发放 ${formatDateTime(record.rewardedAt)}` : '尚未发放'}</small>
                    </td>
                  </tr>
                ))}
                {!loading && invitationRecords.length === 0 && (
                  <tr><td colSpan={8} className="empty-cell">暂无邀请记录</td></tr>
                )}
                {loading && invitationRecords.length === 0 && (
                  <tr><td colSpan={8} className="empty-cell">加载中...</td></tr>
                )}
              </tbody>
            </table>
          </div>

          <footer className="pager">
            <button disabled={invitationRecordPage <= 0 || loading} onClick={() => loadInvitationRecords(invitationRecordPage - 1)}>上一页</button>
            <span>{invitationRecordPage + 1} / {invitationRecordTotalPages}</span>
            <button disabled={invitationRecordPage + 1 >= invitationRecordTotalPages || loading} onClick={() => loadInvitationRecords(invitationRecordPage + 1)}>下一页</button>
          </footer>
          </section>
        </>
      )}

      {activeSection === 'recharge' && (
        <>
          <section className="toolbar favorite-toolbar">
            <div className="settings-title">
              <CircleDollarSign size={18} />
              <span>累计充值活动</span>
            </div>
            <span />
            <button
              className="ghost-btn"
              type="button"
              onClick={() => {
                setEditingRechargeActivity(null);
                setRechargeModalOpen(true);
              }}
            >
              <Plus size={17} />
              新建活动
            </button>
            <button className="ghost-btn" type="button" onClick={loadRechargeActivities} disabled={loading}>
              <CheckCircle2 size={17} />
              刷新
            </button>
          </section>
          <section className="recharge-reward-stats">
            <article className="recharge-reward-total">
              <span>总返利金额</span>
              <strong>{Number(rechargeRewardStats?.totalRewardAmount ?? 0).toFixed(2)}</strong>
            </article>
            <RechargeRewardTrendChart daily={rechargeRewardStats?.daily ?? []} />
          </section>
          <section className="favorite-card-panel recharge-admin-panel">
            <div className="favorite-card-grid">
              {rechargeActivities.map((activity) => (
                <article className="favorite-card recharge-admin-card" key={activity.id}>
                  <div className="favorite-card-link">
                    <div className="favorite-card-main">
                      <div className={`site-icon preset ${activity.enabled ? '' : 'is-muted'}`}>
                        <CircleDollarSign size={20} />
                      </div>
                      <div className="favorite-card-content">
                        <div className="favorite-card-title">
                          <span>{activity.name}</span>
                        </div>
                        <p>{activity.description || '未填写活动说明'}</p>
                      </div>
                    </div>
                  </div>
                  <div className="favorite-card-footer">
                    <span className={`favorite-group-pill ${activity.enabled ? '' : 'is-disabled'}`}>
                      {activity.enabled ? '启用中' : '已停用'} · {activity.tiers.length} 档
                    </span>
                    <div className="favorite-card-actions">
                      <button
                        type="button"
                        className="icon-btn"
                        title="编辑"
                        onClick={() => {
                          setEditingRechargeActivity(activity);
                          setRechargeModalOpen(true);
                        }}
                      >
                        <Pencil size={17} />
                      </button>
                      <button
                        type="button"
                        className="icon-btn"
                        title="删除"
                        onClick={async () => {
                          if (!await confirmDialog({
                            title: '删除充值活动',
                            message: '确认删除这个累计充值活动？',
                            confirmText: '删除',
                            danger: true
                          })) return;
                          await deleteRechargeActivity(activity.id);
                          loadRechargeActivities();
                        }}
                      >
                        <Trash2 size={17} />
                      </button>
                    </div>
                  </div>
                </article>
              ))}
              {!loading && rechargeActivities.length === 0 && (
                <div className="amount-stats-empty">暂无累计充值活动，点击新建活动开始配置。</div>
              )}
            </div>
          </section>

          <section className="table-panel recharge-claims-panel">
            <form
              className="toolbar recharge-claim-toolbar"
              onSubmit={(event) => {
                event.preventDefault();
                loadRechargeActivities(0);
              }}
            >
              <div className="settings-title">
                <KeyRound size={18} />
                <span>用户兑换记录</span>
              </div>
              <div className="search-box">
                <Search size={17} />
                <input
                  value={rechargeClaimKeyword}
                  onChange={(event) => setRechargeClaimKeyword(event.target.value)}
                  placeholder="用户 ID 或历史兑换码"
                />
              </div>
              <select value={rechargeClaimStatus} onChange={(event) => setRechargeClaimStatus(event.target.value)}>
                {rechargeClaimStatusOptions.map((item) => (
                  <option key={item.value || 'all'} value={item.value}>{item.label}</option>
                ))}
              </select>
              <button className="ghost-btn" type="submit" disabled={loading}>
                <Search size={17} />
                查询
              </button>
              <button
                className="ghost-btn"
                type="button"
                disabled={loading}
                onClick={() => {
                  setRechargeClaimKeyword('');
                  setRechargeClaimStatus('');
                  loadRechargeActivities(0, { keyword: '', status: '' });
                }}
              >
                <CheckCircle2 size={17} />
                刷新
              </button>
            </form>

            <table>
              <thead>
                <tr>
                  <th>用户 ID</th>
                  <th>活动</th>
                  <th>门槛</th>
                  <th>奖励</th>
                  <th>状态</th>
                  <th>入账方式</th>
                  <th>时间</th>
                </tr>
              </thead>
              <tbody>
                {rechargeClaims.map((claim) => (
                  <tr key={claim.id}>
                    <td>{claim.userId}</td>
                    <td>
                      <strong>{claim.activityName || `活动 #${claim.activityId}`}</strong>
                      <small>档位 #{claim.tierSort + 1}</small>
                    </td>
                    <td>{Number(claim.thresholdAmount).toFixed(2)}</td>
                    <td>{Number(claim.rewardAmount).toFixed(2)}</td>
                    <td>
                      <span className={`claim-status claim-${claim.status.toLowerCase()}`}>
                        {rechargeClaimStatusText(claim.status)}
                      </span>
                    </td>
                    <td>
                      {claim.redeemCode ? <code>{claim.redeemCode}</code> : <span>管理员调整</span>}
                      {claim.errorMessage && <small className="claim-error">{claim.errorMessage}</small>}
                    </td>
                    <td>{formatDateTime(claim.updatedAt || claim.createdAt)}</td>
                  </tr>
                ))}
                {!loading && rechargeClaims.length === 0 && (
                  <tr>
                    <td colSpan={7} className="empty-cell">暂无用户兑换记录</td>
                  </tr>
                )}
                {loading && rechargeClaims.length === 0 && (
                  <tr>
                    <td colSpan={7} className="empty-cell">加载中...</td>
                  </tr>
                )}
              </tbody>
            </table>

            <footer className="pager">
              <button disabled={rechargeClaimPage <= 0 || loading} onClick={() => loadRechargeActivities(rechargeClaimPage - 1)}>上一页</button>
              <span>{rechargeClaimPage + 1} / {rechargeClaimTotalPages}</span>
              <button disabled={rechargeClaimPage + 1 >= rechargeClaimTotalPages || loading} onClick={() => loadRechargeActivities(rechargeClaimPage + 1)}>下一页</button>
            </footer>
          </section>
        </>
      )}

      {activeSection === 'rates' && (
        <>
          <form className="settings-panel group-rate-panel" onSubmit={saveGroupRateMonitor}>
            <div className="settings-panel-head">
              <div className="settings-title">
                <Globe2 size={18} />
                <span>Sub2API 分组倍率监控</span>
              </div>
              <div className="checkin-actions">
                <button className="ghost-btn" type="button" onClick={refreshGroupRatesNow} disabled={groupRateRefreshing}>
                  <CheckCircle2 size={17} />
                  {groupRateRefreshing ? '刷新中...' : '立即刷新'}
                </button>
                <button className="ghost-btn" type="submit" disabled={groupRateSaving}>
                  <CheckCircle2 size={17} />
                  {groupRateSaving ? '保存中...' : '保存配置'}
                </button>
                {groupRateSaved && <span className="settings-saved">已保存</span>}
              </div>
            </div>

            <div className="group-rate-controls">
              <label className="toggle-row">
                <input
                  type="checkbox"
                  checked={groupRateDraft.enabled}
                  onChange={(event) => patchGroupRateDraft({ enabled: event.target.checked })}
                />
                启用定时拉取
              </label>
              <label>
                刷新间隔秒数
                <input
                  type="number"
                  min="60"
                  max="86400"
                  step="60"
                  value={groupRateDraft.refreshIntervalSeconds}
                  onChange={(event) => patchGroupRateDraft({ refreshIntervalSeconds: Number(event.target.value) })}
                />
              </label>
              <button
                className="ghost-btn"
                type="button"
                onClick={() => patchGroupRateDraft({ monitoredGroupIds: [] })}
              >
                监控全部分组
              </button>
              <button
                className="ghost-btn"
                type="button"
                onClick={() => patchGroupRateDraft({ monitoredGroupIds: groupRateMonitor.groups.map((group) => group.groupId) })}
                disabled={groupRateMonitor.groups.length === 0}
              >
                自定义监控
              </button>
            </div>

            <div className="group-rate-table">
              <div className="group-rate-row group-rate-head">
                <span>分组</span>
                <span>当前倍率</span>
                <span>最后拉取</span>
                <span>监控</span>
                <span>公开</span>
                <span>操作</span>
              </div>
              {groupRateMonitor.groups.map((group) => {
                const monitorAll = groupRateDraft.monitoredGroupIds.length === 0;
                const monitored = monitorAll || groupRateDraft.monitoredGroupIds.includes(group.groupId);
                const publicVisible = groupRateDraft.publicGroupIds.includes(group.groupId);
                return (
                  <div className="group-rate-row" key={group.groupId}>
                    <div>
                      <strong>{group.groupName}</strong>
                      <small>{group.groupId}</small>
                    </div>
                    <input
                      className="compact-rate-input"
                      type="number"
                      min="0.000001"
                      step="0.000001"
                      value={Number(group.rateMultiplier).toFixed(6)}
                      readOnly
                    />
                    <span>{group.lastSeenAt || '-'}</span>
                    <label className="toggle-row compact">
                      <input
                        type="checkbox"
                        checked={monitored}
                        disabled={monitorAll}
                        onChange={(event) => toggleGroupRateId('monitoredGroupIds', group.groupId, event.target.checked)}
                      />
                      {monitorAll ? '全部' : '启用'}
                    </label>
                    <label className="toggle-row compact">
                      <input
                        type="checkbox"
                        checked={publicVisible}
                        onChange={(event) => toggleGroupRateId('publicGroupIds', group.groupId, event.target.checked)}
                      />
                      展示
                    </label>
                    <button
                      className="icon-btn"
                      type="button"
                      title="倍率日志"
                      onClick={() => setEditingGroupRate(group)}
                    >
                      <Pencil size={16} />
                    </button>
                  </div>
                );
              })}
              {!loading && groupRateMonitor.groups.length === 0 && (
                <div className="amount-stats-empty">暂无分组快照，点击立即刷新后会从 Sub2API 拉取。</div>
              )}
            </div>
          </form>

          <section className="user-info-panel">
            <div className="settings-title">
              <CircleDollarSign size={18} />
              <span>倍率变化阶梯图</span>
            </div>
            <RateLineChart series={groupRateMonitor.series} />
          </section>

          <section className="user-info-panel group-rate-log-panel">
            <div className="settings-title">
              <Pencil size={18} />
              <span>最近倍率变动日志</span>
            </div>
            <div className="group-rate-log-table">
              <div className="group-rate-log-row group-rate-head">
                <span>分组</span>
                <span>旧倍率</span>
                <span>新倍率</span>
                <span>来源</span>
                <span>变动时间</span>
                <span>公开</span>
                <span>操作</span>
              </div>
              {(groupRateMonitor.logs ?? []).map((entry) => {
                const draft = groupRateLogDrafts[entry.id] ?? {
                  oldRate: Number(entry.oldRate).toFixed(6),
                  newRate: Number(entry.newRate).toFixed(6),
                  createdAt: toDateTimeLocal(entry.createdAt),
                  publicVisible: entry.publicVisible
                };
                return (
                  <div className="group-rate-log-row" key={entry.id}>
                    <div>
                      <strong>{entry.groupName}</strong>
                      <small>{entry.groupId}</small>
                    </div>
                    <input
                      className="compact-rate-input"
                      type="number"
                      min="0.000001"
                      step="0.000001"
                      value={draft.oldRate}
                      onChange={(event) => patchGroupRateLogDraft(entry.id, { oldRate: event.target.value })}
                    />
                    <input
                      className="compact-rate-input"
                      type="number"
                      min="0.000001"
                      step="0.000001"
                      value={draft.newRate}
                      onChange={(event) => patchGroupRateLogDraft(entry.id, { newRate: event.target.value })}
                    />
                    <span>{entry.source}</span>
                    <input
                      type="datetime-local"
                      value={draft.createdAt}
                      onChange={(event) => patchGroupRateLogDraft(entry.id, { createdAt: event.target.value })}
                    />
                    <label className="toggle-row compact">
                      <input
                        type="checkbox"
                        checked={draft.publicVisible}
                        onChange={(event) => patchGroupRateLogDraft(entry.id, { publicVisible: event.target.checked })}
                      />
                      展示
                    </label>
                    <button
                      className="icon-btn"
                      type="button"
                      title="保存日志"
                      disabled={groupRateEditingKey === `log:${entry.id}`}
                      onClick={() => saveGroupRateLog(entry.id)}
                    >
                      <CheckCircle2 size={16} />
                    </button>
                  </div>
                );
              })}
              {!loading && (groupRateMonitor.logs ?? []).length === 0 && (
                <div className="amount-stats-empty">暂无倍率变动日志。</div>
              )}
            </div>
          </section>
        </>
      )}

      {activeSection === 'system' && (
        <form className="settings-panel" onSubmit={saveCheckInSettings}>
          <div className="settings-panel-head">
            <div className="settings-title">
              <Settings2 size={18} />
              <span>系统设置</span>
            </div>
            <button className="ghost-btn" type="submit" disabled={settingsSaving || !hasSettingsChanges}>
              <CheckCircle2 size={17} />
              {settingsSaving ? '保存中...' : '保存'}
            </button>
            {settingsSaved && <span className="settings-saved">已保存</span>}
          </div>

          <div className="sub2api-editor standalone system-update-panel">
            <div className="tier-editor-head">
              <span>版本更新</span>
              <div className="system-update-actions">
                {systemUpdate?.releaseUrl && (
                  <a className="ghost-btn" href={systemUpdate.releaseUrl} target="_blank" rel="noreferrer">
                    <ExternalLink size={16} />
                    Release
                  </a>
                )}
                <button className="ghost-btn" type="button" onClick={() => checkSystemUpdate(true)} disabled={systemUpdateChecking}>
                  <CheckCircle2 size={17} />
                  {systemUpdateChecking ? '检测中...' : '检测更新'}
                </button>
                <button
                  className="primary-btn"
                  type="button"
                  onClick={updateSystemVersion}
                  disabled={systemUpdating || !systemUpdate?.updateAvailable || !systemUpdate?.updateEnabled}
                  title={systemUpdate?.updateEnabled ? '' : '当前部署未启用后台更新'}
                >
                  {systemUpdating ? '更新中...' : '立即更新'}
                </button>
              </div>
            </div>
            <div className="system-update-grid">
              <div>
                <span>当前版本</span>
                <strong>{systemUpdate?.currentVersion || '-'}</strong>
              </div>
              <div>
                <span>最新版本</span>
                <strong>{systemUpdate?.latestVersion || '-'}</strong>
              </div>
              <div>
                <span>更新状态</span>
                <strong className={systemUpdate?.updateAvailable ? 'is-warning' : 'is-ok'}>{systemUpdate?.message || '尚未检测'}</strong>
              </div>
              <div>
                <span>发布时间</span>
                <strong>{formatOptionalDate(systemUpdate?.publishedAt)}</strong>
              </div>
            </div>
            <div className="system-update-note">
              <span>Release 标准：{systemUpdate?.repository || 'hepingan11/sub2-Expansion'} 的 latest release。</span>
              {!systemUpdate?.updateEnabled && (
                <span>Docker 部署默认自动启用；未启用时请检查 SYSTEM_UPDATE_ENABLED、项目目录挂载和 Docker Socket。</span>
              )}
            </div>
            {systemUpdateOutput && (
              <pre className="system-update-output">{systemUpdateOutput}</pre>
            )}
          </div>

          <div className="sub2api-editor standalone public-url-editor">
            <div className="tier-editor-head">
              <div className="settings-title">
                <ExternalLink size={18} />
                <span>公开访问地址</span>
              </div>
            </div>
            <label>
              前端公开地址
              <input
                value={frontendPublicUrlDraft}
                onChange={(event) => {
                  setFrontendPublicUrlDraft(event.target.value);
                  setSettingsSaved(false);
                }}
                placeholder="https://your-public-frontend"
              />
            </label>
            <div className="system-update-note">
              <span>Telegram 和其它社交平台绑定链接会使用这个地址。留空时回退到环境变量 FRONTEND_PUBLIC_URL；仍为空时尝试使用当前请求来源。</span>
            </div>
          </div>

          <div className="sub2api-editor standalone invitation-system-panel">
            <div className="tier-editor-head">
              <div className="settings-title">
                <UserPlus size={18} />
                <span>邀请规则</span>
              </div>
              <span className={`invitation-status ${invitationSettingsDraft.afterTime && Number(invitationSettingsDraft.amount) > 0 ? 'is-active' : ''}`}>
                {invitationSettingsDraft.afterTime && Number(invitationSettingsDraft.amount) > 0 ? '已启用' : '未启用'}
              </span>
            </div>
            <div className="sub2api-grid invitation-system-grid">
              <label>
                新用户注册时间限制
                <input
                  type="datetime-local"
                  value={invitationSettingsDraft.afterTime}
                  onChange={(event) => {
                    setInvitationSettingsDraft((current) => ({ ...current, afterTime: event.target.value }));
                    setSettingsSaved(false);
                  }}
                />
              </label>
              <label>
                邀请人获得余额
                <input
                  type="number"
                  min="0"
                  step="0.01"
                  value={invitationSettingsDraft.amount}
                  onChange={(event) => {
                    setInvitationSettingsDraft((current) => ({ ...current, amount: Number(event.target.value) }));
                    setSettingsSaved(false);
                  }}
                  placeholder="例如 5.00"
                />
              </label>
            </div>
            <div className="system-update-note invitation-rule-note">
              <span>只有 Sub2API 账号创建时间严格晚于该时间，且通过群机器人链接完成绑定，邀请人才会获得余额。</span>
              <span>时间和金额同时留空或设为 0 时，邀请奖励功能关闭。</span>
            </div>
          </div>

          <div className="sub2api-editor standalone">
            <div className="tier-editor-head">
              <span>后台管理员</span>
            </div>
            <div className="sub2api-grid">
              <label>
                登录账号
                <input
                  value={adminSettingsDraft.username}
                  onChange={(event) => {
                    setAdminSettingsDraft((current) => ({ ...current, username: event.target.value }));
                    setSettingsSaved(false);
                  }}
                  placeholder="admin"
                />
              </label>
              <label>
                登录密码
                <input
                  type="password"
                  value={adminSettingsDraft.password ?? ''}
                  onChange={(event) => {
                    setAdminSettingsDraft((current) => ({ ...current, password: event.target.value }));
                    setSettingsSaved(false);
                  }}
                  placeholder={adminSettings.passwordSet ? '已设置，留空则不修改' : '输入新的后台登录密码'}
                />
              </label>
            </div>
          </div>

          <div className="sub2api-editor standalone telegram-editor">
            <div className="tier-editor-head">
              <div className="settings-title">
                <Globe2 size={18} />
                <span>Telegram Bot</span>
              </div>
              <div className="system-update-actions">
                <span className={`invitation-status ${telegramDraft.enabled && telegram.botTokenSet ? 'is-active' : ''}`}>
                  {telegramDraft.enabled && telegram.botTokenSet ? (telegram.botUsername ? `@${telegram.botUsername}` : '已配置') : '未启用'}
                </span>
                <button
                  className="ghost-btn"
                  type="button"
                  onClick={connectTelegram}
                  disabled={telegramConnecting || settingsSaving || !telegramDraft.enabled}
                >
                  <CheckCircle2 size={17} />
                  {telegramConnecting ? '连接中...' : '连接测试'}
                </button>
              </div>
            </div>
            <div className="telegram-control-line">
              <label className="toggle-row">
                <input
                  type="checkbox"
                  checked={telegramDraft.enabled}
                  onChange={(event) => {
                    setTelegramDraft((current) => ({ ...current, enabled: event.target.checked }));
                    setSettingsSaved(false);
                  }}
                />
                启用 Telegram Bot 长轮询
              </label>
              <span>{telegram.botTokenSet ? 'Bot Token 已保存，留空不会修改。' : '尚未保存 Bot Token。'}</span>
            </div>
            <div className="telegram-control-line">
              <label className="toggle-row">
                <input
                  type="checkbox"
                  checked={telegramDraft.membershipCheckEnabled}
                  onChange={(event) => {
                    setTelegramDraft((current) => ({ ...current, membershipCheckEnabled: event.target.checked }));
                    setSettingsSaved(false);
                  }}
                />
                绑定前检查指定群成员身份
              </label>
              <span>Bot 必须是目标群管理员。</span>
            </div>
            <div className="sub2api-grid">
              <label>
                Bot Token
                <input
                  type="password"
                  value={telegramDraft.botToken ?? ''}
                  onChange={(event) => {
                    setTelegramDraft((current) => ({ ...current, botToken: event.target.value }));
                    setSettingsSaved(false);
                  }}
                  placeholder={telegram.botTokenSet ? '已设置，留空则不修改' : '从 BotFather 获取的 Token'}
                  autoComplete="off"
                />
              </label>
              <label>
                Telegram API 地址
                <input
                  value={telegramDraft.apiBaseUrl}
                  onChange={(event) => {
                    setTelegramDraft((current) => ({ ...current, apiBaseUrl: event.target.value }));
                    setSettingsSaved(false);
                  }}
                  placeholder="https://api.telegram.org"
                />
              </label>
              <label>
                轮询间隔秒数
                <input
                  type="number"
                  min="1"
                  step="1"
                  value={telegramDraft.pollIntervalSeconds}
                  onChange={(event) => {
                    setTelegramDraft((current) => ({ ...current, pollIntervalSeconds: Number(event.target.value) }));
                    setSettingsSaved(false);
                  }}
                />
              </label>
              <label>
                Bot Username
                <input value={telegram.botUsername ? `@${telegram.botUsername}` : ''} readOnly placeholder="连接测试后显示" />
              </label>
              <label>
                目标群 Chat ID
                <input
                  value={telegramDraft.requiredGroupChatId}
                  onChange={(event) => {
                    setTelegramDraft((current) => ({ ...current, requiredGroupChatId: event.target.value }));
                    setSettingsSaved(false);
                  }}
                  placeholder="-1001234567890"
                />
              </label>
              <label>
                加群链接
                <input
                  value={telegramDraft.groupJoinUrl}
                  onChange={(event) => {
                    setTelegramDraft((current) => ({ ...current, groupJoinUrl: event.target.value }));
                    setSettingsSaved(false);
                  }}
                  placeholder="https://t.me/your_group"
                />
              </label>
              <label>
                绑定凭证有效期（分钟）
                <input
                  type="number"
                  min="1"
                  max="1440"
                  step="1"
                  value={telegramDraft.bindingTokenTtlMinutes}
                  onChange={(event) => {
                    setTelegramDraft((current) => ({ ...current, bindingTokenTtlMinutes: Number(event.target.value) }));
                    setSettingsSaved(false);
                  }}
                />
              </label>
            </div>
            <div className="system-update-note">
              <span>保存配置后点击连接测试，后端会调用 Telegram getMe 并立即启动或重启 Bot 轮询。</span>
              <span>启用入群校验后，/start 邀请链接和 /bind 都会先检查群成员身份，通过后才签发短期绑定凭证。</span>
            </div>
          </div>

          <div className="sub2api-editor standalone">
            <div className="tier-editor-head">
              <span>Sub2API 远程配置</span>
            </div>
            <div className="sub2api-grid">
              <label>
                远程地址
                <input
                  value={sub2apiDraft.baseUrl}
                  onChange={(event) => updateSub2APIDraft('baseUrl', event.target.value, setSub2apiDraft, setSettingsSaved)}
                  placeholder="https://your-sub2api-host"
                />
              </label>
              <label>
                超时秒数
                <input
                  type="number"
                  min="1"
                  step="1"
                  value={sub2apiDraft.timeoutSeconds}
                  onChange={(event) => updateSub2APIDraft('timeoutSeconds', Number(event.target.value), setSub2apiDraft, setSettingsSaved)}
                />
              </label>
              <label>
                认证方式
                <select
                  value={sub2apiDraft.authMode}
                  onChange={(event) => updateSub2APIDraft('authMode', event.target.value as Sub2APISettings['authMode'], setSub2apiDraft, setSettingsSaved)}
                >
                  <option value="password">管理员账号密码</option>
                  <option value="admin_api_key">Admin API Key</option>
                </select>
              </label>
              <label>
                管理员邮箱
                <input
                  value={sub2apiDraft.adminEmail}
                  onChange={(event) => updateSub2APIDraft('adminEmail', event.target.value, setSub2apiDraft, setSettingsSaved)}
                  placeholder="admin@example.com"
                />
              </label>
              <label>
                管理员密码
                <input
                  type="password"
                  value={sub2apiDraft.adminPassword ?? ''}
                  onChange={(event) => updateSub2APIDraft('adminPassword', event.target.value, setSub2apiDraft, setSettingsSaved)}
                  placeholder={sub2api.adminPasswordSet ? '已设置，留空则不修改' : ''}
                />
              </label>
              <label>
                Admin API Key
                <input
                  value={sub2apiDraft.adminApiKey ?? ''}
                  onChange={(event) => updateSub2APIDraft('adminApiKey', event.target.value, setSub2apiDraft, setSettingsSaved)}
                  placeholder={sub2api.adminApiKeySet ? '已设置，留空则不修改' : '可选，优先级最高'}
                />
              </label>
            </div>
          </div>
        </form>
      )}

      {modalOpen && (
        <CodeModal
          code={editing}
          onClose={() => setModalOpen(false)}
          onSaved={() => {
            setModalOpen(false);
            load(page);
          }}
        />
      )}

      {importOpen && (
        <ImportModal
          onClose={() => setImportOpen(false)}
          onImported={() => {
            setImportOpen(false);
            load(0);
          }}
        />
      )}

      {favoriteModalOpen && (
        <FavoriteSiteModal
          site={editingFavorite}
          groups={favoriteGroups}
          onClose={() => setFavoriteModalOpen(false)}
          onSaved={() => {
            setFavoriteModalOpen(false);
            loadFavoriteSites(editingFavorite ? favoritePage : 0);
          }}
          onDeleted={() => {
            setFavoriteModalOpen(false);
            setEditingFavorite(null);
            loadFavoriteSites(favoritePage);
          }}
        />
      )}

      {rechargeModalOpen && (
        <RechargeActivityModal
          activity={editingRechargeActivity}
          onClose={() => setRechargeModalOpen(false)}
          onSaved={() => {
            setRechargeModalOpen(false);
            setEditingRechargeActivity(null);
            loadRechargeActivities();
          }}
        />
      )}

      {editingGroupRate && (
        <GroupRateLogModal
          group={editingGroupRate}
          onClose={() => setEditingGroupRate(null)}
          onChanged={async () => {
            applyGroupRateMonitor(await fetchSub2APIGroupRateMonitor());
          }}
        />
      )}
      </section>
    </main>
  );
}

function SiteIcon({ site }: { site: FavoriteSite }) {
  const preset = findFavoriteIconPreset(site.icon);
  if (preset) {
    const Icon = preset.icon;
    return (
      <div className="site-icon preset" title={preset.label} aria-hidden="true">
        <Icon size={18} />
      </div>
    );
  }
  const emojiPreset = findFavoriteEmojiPreset(site.icon);
  if (emojiPreset) {
    return (
      <div className="site-icon emoji" title={emojiPreset.label} aria-hidden="true">
        {emojiPreset.emoji}
      </div>
    );
  }
  if (site.icon) {
    return <img className="site-icon" src={site.icon} alt="" loading="lazy" />;
  }
  return (
    <div className="site-icon fallback" aria-hidden="true">
      {site.name.trim().slice(0, 1).toUpperCase() || <Bookmark size={15} />}
    </div>
  );
}

function GroupRateLogModal({
  group,
  onClose,
  onChanged
}: {
  group: Sub2APIGroupRateGroup;
  onClose: () => void;
  onChanged: () => Promise<void>;
}) {
  const [logs, setLogs] = useState<Sub2APIGroupRateLog[]>([]);
  const [drafts, setDrafts] = useState<Record<number, { oldRate: string; newRate: string; createdAt: string; publicVisible: boolean }>>({});
  const [newRate, setNewRate] = useState('');
  const [newTime, setNewTime] = useState(() => toDateTimeLocal(new Date().toISOString()));
  const [newPublicVisible, setNewPublicVisible] = useState(group.publicVisible);
  const [loading, setLoading] = useState(true);
  const [savingKey, setSavingKey] = useState<string | null>(null);

  function applyLogs(nextLogs: Sub2APIGroupRateLog[]) {
    setLogs(nextLogs);
    setDrafts(Object.fromEntries(nextLogs.map((entry) => [entry.id, {
      oldRate: Number(entry.oldRate).toFixed(6),
      newRate: Number(entry.newRate).toFixed(6),
      createdAt: toDateTimeLocal(entry.createdAt),
      publicVisible: entry.publicVisible
    }])));
  }

  async function loadLogs() {
    setLoading(true);
    try {
      applyLogs(await fetchSub2APIGroupRateLogs(group.groupId));
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '加载倍率日志失败');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadLogs();
  }, [group.groupId]);

  function patchDraft(id: number, patch: Partial<{ oldRate: string; newRate: string; createdAt: string; publicVisible: boolean }>) {
    setDrafts((current) => ({ ...current, [id]: { ...current[id], ...patch } }));
  }

  async function addLog(event: FormEvent) {
    event.preventDefault();
    const parsedRate = Number(newRate);
    if (!Number.isFinite(parsedRate) || parsedRate <= 0) {
      notifyError('倍率必须是大于 0 的数字');
      return;
    }
    setSavingKey('new');
    try {
      applyLogs(await createSub2APIGroupRateLog(group.groupId, {
        rateMultiplier: parsedRate,
        createdAt: newTime,
        publicVisible: newPublicVisible
      }));
      setNewRate('');
      setNewTime(toDateTimeLocal(new Date().toISOString()));
      await onChanged();
      notifySuccess('倍率日志已新增');
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '新增倍率日志失败');
    } finally {
      setSavingKey(null);
    }
  }

  async function saveLog(entry: Sub2APIGroupRateLog) {
    const draft = drafts[entry.id];
    if (!draft) return;
    const oldRate = Number(draft.oldRate);
    const nextRate = Number(draft.newRate);
    if (!Number.isFinite(oldRate) || oldRate <= 0 || !Number.isFinite(nextRate) || nextRate <= 0) {
      notifyError('旧倍率和新倍率都必须是大于 0 的数字');
      return;
    }
    setSavingKey(`save:${entry.id}`);
    try {
      await updateSub2APIGroupRateLog(entry.id, {
        oldRate,
        newRate: nextRate,
        createdAt: draft.createdAt,
        publicVisible: draft.publicVisible
      });
      applyLogs(await fetchSub2APIGroupRateLogs(group.groupId));
      await onChanged();
      notifySuccess('倍率日志已保存');
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '保存倍率日志失败');
    } finally {
      setSavingKey(null);
    }
  }

  async function removeLog(entry: Sub2APIGroupRateLog) {
    if (!await confirmDialog({
      title: '删除倍率日志',
      message: `确认删除 ${entry.groupName} 在 ${entry.createdAt} 的倍率记录？`,
      confirmText: '删除',
      danger: true
    })) {
      return;
    }
    setSavingKey(`delete:${entry.id}`);
    try {
      applyLogs(await deleteSub2APIGroupRateLog(entry.id));
      await onChanged();
      notifySuccess('倍率日志已删除');
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '删除倍率日志失败');
    } finally {
      setSavingKey(null);
    }
  }

  return (
    <div className="modal-backdrop">
      <div className="modal group-rate-log-modal">
        <div className="modal-head">
          <div>
            <span className="eyebrow">Rate History</span>
            <h2>{group.groupName} 倍率日志</h2>
          </div>
          <button type="button" className="icon-btn" onClick={onClose} title="关闭">
            <X size={18} />
          </button>
        </div>

        <form className="group-rate-log-create" onSubmit={addLog}>
          <label>
            变动时间
            <input type="datetime-local" value={newTime} onChange={(event) => setNewTime(event.target.value)} required />
          </label>
          <label>
            新倍率
            <input
              type="number"
              min="0.000001"
              step="0.000001"
              value={newRate}
              onChange={(event) => setNewRate(event.target.value)}
              placeholder={Number(group.rateMultiplier).toFixed(6)}
              required
            />
          </label>
          <label className="toggle-row compact">
            <input type="checkbox" checked={newPublicVisible} onChange={(event) => setNewPublicVisible(event.target.checked)} />
            公开
          </label>
          <button className="primary-btn" type="submit" disabled={savingKey === 'new'}>
            <Plus size={17} />
            {savingKey === 'new' ? '新增中...' : '新增日志'}
          </button>
        </form>

        <div className="group-rate-log-table modal-log-table">
          <div className="group-rate-log-row modal-log-row group-rate-head">
            <span>变动时间</span>
            <span>旧倍率</span>
            <span>新倍率</span>
            <span>来源</span>
            <span>公开</span>
            <span>操作</span>
          </div>
          {logs.map((entry) => {
            const draft = drafts[entry.id] ?? {
              oldRate: Number(entry.oldRate).toFixed(6),
              newRate: Number(entry.newRate).toFixed(6),
              createdAt: toDateTimeLocal(entry.createdAt),
              publicVisible: entry.publicVisible
            };
            return (
              <div className="group-rate-log-row modal-log-row" key={entry.id}>
                <input type="datetime-local" value={draft.createdAt} onChange={(event) => patchDraft(entry.id, { createdAt: event.target.value })} />
                <input className="compact-rate-input" type="number" min="0.000001" step="0.000001" value={draft.oldRate} onChange={(event) => patchDraft(entry.id, { oldRate: event.target.value })} />
                <input className="compact-rate-input" type="number" min="0.000001" step="0.000001" value={draft.newRate} onChange={(event) => patchDraft(entry.id, { newRate: event.target.value })} />
                <span>{entry.source}</span>
                <label className="toggle-row compact">
                  <input type="checkbox" checked={draft.publicVisible} onChange={(event) => patchDraft(entry.id, { publicVisible: event.target.checked })} />
                  展示
                </label>
                <div className="modal-log-actions">
                  <button className="icon-btn" type="button" title="保存日志" disabled={savingKey === `save:${entry.id}`} onClick={() => saveLog(entry)}>
                    <CheckCircle2 size={16} />
                  </button>
                  <button className="icon-btn danger-icon" type="button" title="删除日志" disabled={savingKey === `delete:${entry.id}`} onClick={() => removeLog(entry)}>
                    <Trash2 size={16} />
                  </button>
                </div>
              </div>
            );
          })}
          {!loading && logs.length === 0 && (
            <div className="amount-stats-empty">暂无倍率日志，可以在上方新增第一条记录。</div>
          )}
          {loading && <div className="amount-stats-empty">正在加载倍率日志</div>}
        </div>
      </div>
    </div>
  );
}

function FavoriteIconPicker({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const [open, setOpen] = useState(false);
  const [customMode, setCustomMode] = useState(isCustomFavoriteIcon(value));
  const preset = findFavoriteIconPreset(value);
  const emojiPreset = findFavoriteEmojiPreset(value);

  function select(value: string) {
    onChange(value);
    setCustomMode(false);
    setOpen(false);
  }

  return (
    <div className="icon-picker">
      <span className="field-label">图标</span>
      <button type="button" className="icon-picker-trigger" onClick={() => setOpen((current) => !current)}>
        <span className="icon-picker-current">
          <IconPreview value={value} />
          <span>{iconLabel(value)}</span>
        </span>
        <ChevronDown size={17} />
      </button>
      {open && (
        <div className="icon-picker-menu">
          <div className="icon-picker-section">
            <span>基础</span>
            <div className="icon-preset-grid compact">
              <button type="button" className={value === '' ? 'is-selected' : ''} onClick={() => select('')}>
                <span className="empty-icon-dot" />
                无图标
              </button>
              <button
                type="button"
                className={customMode || isCustomFavoriteIcon(value) ? 'is-selected' : ''}
                onClick={() => {
                  setCustomMode(true);
                  onChange(isCustomFavoriteIcon(value) ? value : '');
                }}
              >
                <ExternalLink size={17} />
                自定义地址
              </button>
            </div>
          </div>

          <div className="icon-picker-section">
            <span>预设图标</span>
            <div className="icon-preset-grid compact">
              {favoriteIconPresets.map((item) => {
                const Icon = item.icon;
                return (
                  <button
                    key={item.value}
                    type="button"
                    className={preset?.value === item.value ? 'is-selected' : ''}
                    onClick={() => select(item.value)}
                    title={item.label}
                  >
                    <Icon size={18} />
                    {item.label}
                  </button>
                );
              })}
            </div>
          </div>

          <div className="icon-picker-section">
            <span>Emoji 图标</span>
            <div className="icon-preset-grid emoji-grid compact">
              {favoriteEmojiPresets.map((item) => (
                <button
                  key={item.value}
                  type="button"
                  className={emojiPreset?.value === item.value ? 'is-selected' : ''}
                  onClick={() => select(item.value)}
                  title={item.label}
                >
                  <span className="emoji-mark">{item.emoji}</span>
                  {item.label}
                </button>
              ))}
            </div>
          </div>

          {customMode && (
            <label className="icon-custom-url">
              自定义图标地址
              <input
                value={isCustomFavoriteIcon(value) ? value : ''}
                onChange={(event) => onChange(event.target.value)}
                placeholder="https://example.com/favicon.ico"
              />
            </label>
          )}
        </div>
      )}
    </div>
  );
}

function IconPreview({ value }: { value: string }) {
  const preset = findFavoriteIconPreset(value);
  if (preset) {
    const Icon = preset.icon;
    return <Icon size={18} />;
  }
  const emojiPreset = findFavoriteEmojiPreset(value);
  if (emojiPreset) {
    return <span className="emoji-mark">{emojiPreset.emoji}</span>;
  }
  if (isCustomFavoriteIcon(value)) {
    return <ExternalLink size={17} />;
  }
  return <span className="empty-icon-dot" />;
}

function findFavoriteIconPreset(value: string) {
  return favoriteIconPresets.find((item) => item.value === value);
}

function findFavoriteEmojiPreset(value: string) {
  return favoriteEmojiPresets.find((item) => item.value === value);
}

function isCustomFavoriteIcon(value: string) {
  return value !== '' && !findFavoriteIconPreset(value) && !findFavoriteEmojiPreset(value);
}

function iconLabel(value: string) {
  const preset = findFavoriteIconPreset(value);
  if (preset) return preset.label;
  const emojiPreset = findFavoriteEmojiPreset(value);
  if (emojiPreset) return emojiPreset.label;
  if (isCustomFavoriteIcon(value)) return '自定义地址';
  return '无图标';
}

function FavoriteSiteModal({ site, groups, onClose, onSaved, onDeleted }: { site: FavoriteSite | null; groups: string[]; onClose: () => void; onSaved: () => void; onDeleted: () => void }) {
  const [form, setForm] = useState<FavoriteSitePayload>({
    icon: site?.icon ?? '',
    url: site?.url ?? '',
    name: site?.name ?? '',
    description: site?.description ?? '',
    sort: site?.sort ?? 0,
    group: site?.group ?? ''
  });
  const initialGroupMode = site?.group && !groups.includes(site.group) ? '__new__' : (site?.group ?? '');
  const [groupChoice, setGroupChoice] = useState(initialGroupMode);
  const [newGroup, setNewGroup] = useState(initialGroupMode === '__new__' ? site?.group ?? '' : '');
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);

  function patch<K extends keyof FavoriteSitePayload>(key: K, value: FavoriteSitePayload[K]) {
    setForm((current) => ({ ...current, [key]: value }));
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError('');
    try {
      const selectedGroup = groupChoice === '__new__' ? newGroup : groupChoice;
      const payload = {
        ...form,
        group: selectedGroup.trim(),
        sort: Number(form.sort)
      };
      if (site) {
        await updateFavoriteSite(site.id, payload);
      } else {
        await createFavoriteSite(payload);
      }
      onSaved();
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '保存失败');
    } finally {
      setSaving(false);
    }
  }

  async function removeSite() {
    if (!site || !await confirmDialog({
      title: '删除收藏网站',
      message: '确认删除这个收藏网站？',
      confirmText: '删除',
      danger: true
    })) {
      return;
    }
    setDeleting(true);
    setError('');
    try {
      await deleteFavoriteSite(site.id);
      onDeleted();
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '删除失败');
    } finally {
      setDeleting(false);
    }
  }

  return (
    <div className="modal-backdrop">
      <form className="modal favorite-modal" onSubmit={submit}>
        <div className="modal-head">
          <div>
            <span className="eyebrow">{site ? 'Edit Site' : 'New Site'}</span>
            <h2>{site ? '编辑收藏网站' : '新增收藏网站'}</h2>
          </div>
          <button type="button" className="icon-btn" onClick={onClose} title="关闭">
            <X size={18} />
          </button>
        </div>

        <label>
          网站名称
          <input value={form.name} onChange={(event) => patch('name', event.target.value)} maxLength={100} required />
        </label>
        <label>
          URL
          <input value={form.url} onChange={(event) => patch('url', event.target.value)} placeholder="https://example.com" required />
        </label>
        <FavoriteIconPicker value={form.icon} onChange={(value) => patch('icon', value)} />
        <label>
          简介
          <textarea
            className="compact-textarea"
            value={form.description}
            onChange={(event) => patch('description', event.target.value)}
            maxLength={500}
            rows={3}
          />
        </label>
        <div className="modal-grid two">
          <div className="group-editor">
            <label>
              分组
              <select
                value={groupChoice}
                onChange={(event) => {
                  const value = event.target.value;
                  setGroupChoice(value);
                  if (value !== '__new__') {
                    patch('group', value);
                  }
                }}
              >
                <option value="">不分组</option>
                {groups.map((group) => (
                  <option key={group} value={group}>{group}</option>
                ))}
                <option value="__new__">新建分组</option>
              </select>
            </label>
            {groupChoice === '__new__' && (
              <label>
                新分组名称
                <input
                  value={newGroup}
                  onChange={(event) => {
                    setNewGroup(event.target.value);
                    patch('group', event.target.value);
                  }}
                  maxLength={100}
                  placeholder="工具 / 文档 / 常用"
                />
              </label>
            )}
          </div>
          <label>
            排序
            <input
              type="number"
              min="0"
              step="1"
              value={form.sort}
              onChange={(event) => patch('sort', Number(event.target.value))}
              required
            />
          </label>
        </div>

        {error && <div className="error-line">{error}</div>}
        <div className="modal-actions">
          {site && (
            <button className="danger-btn" type="button" disabled={deleting || saving} onClick={removeSite}>
              <Trash2 size={17} />
              {deleting ? '删除中...' : '删除'}
            </button>
          )}
          <button className="primary-btn" type="submit" disabled={saving || deleting}>
            <CheckCircle2 size={18} />
            {saving ? '保存中...' : '保存'}
          </button>
        </div>
      </form>
    </div>
  );
}

function RechargeActivityModal({ activity, onClose, onSaved }: { activity: RechargeActivity | null; onClose: () => void; onSaved: () => void }) {
  const [name, setName] = useState(activity?.name ?? '');
  const [description, setDescription] = useState(activity?.description ?? '');
  const [enabled, setEnabled] = useState(activity?.enabled ?? true);
  const [startAt, setStartAt] = useState(toDateTimeLocal(activity?.startAt));
  const [endAt, setEndAt] = useState(toDateTimeLocal(activity?.endAt));
  const [tiers, setTiers] = useState(() => toRechargeTierDrafts(activity?.tiers));
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);

  function updateTier(index: number, key: keyof RechargeRewardTierDraft, value: string) {
    setTiers((current) => current.map((tier, currentIndex) => (
      currentIndex === index ? { ...tier, [key]: value } : tier
    )));
  }

  function removeTier(index: number) {
    setTiers((current) => current.filter((_, currentIndex) => currentIndex !== index));
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError('');
    try {
      const payload = parseRechargeActivityPayload({ name, description, enabled, startAt, endAt, tiers });
      if (typeof payload === 'string') {
        notifyError(payload);
        return;
      }
      if (activity) {
        await updateRechargeActivity(activity.id, payload);
      } else {
        await createRechargeActivity(payload);
      }
      onSaved();
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '保存充值活动失败');
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="modal-backdrop">
      <form className="modal recharge-modal" onSubmit={submit}>
        <div className="modal-head">
          <div>
            <span className="eyebrow">{activity ? 'Edit Campaign' : 'New Campaign'}</span>
            <h2>{activity ? '编辑累计充值活动' : '新建累计充值活动'}</h2>
          </div>
          <button type="button" className="icon-btn" onClick={onClose} title="关闭">
            <X size={18} />
          </button>
        </div>

        <label>
          活动名称
          <input value={name} onChange={(event) => setName(event.target.value)} maxLength={120} required />
        </label>
        <label>
          活动说明
          <textarea
            className="compact-textarea"
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            rows={3}
          />
        </label>
        <div className="modal-grid two">
          <label>
            开始时间
            <input type="datetime-local" value={startAt} onChange={(event) => setStartAt(event.target.value)} />
          </label>
          <label>
            结束时间
            <input type="datetime-local" value={endAt} onChange={(event) => setEndAt(event.target.value)} />
          </label>
        </div>
        <label className="toggle-row">
          <input type="checkbox" checked={enabled} onChange={(event) => setEnabled(event.target.checked)} />
          启用活动
        </label>

        <div className="recharge-tier-editor">
          <div className="tier-editor-head">
            <span>奖励档位</span>
            <button
              type="button"
              className="ghost-btn"
              onClick={() => setTiers((current) => [...current, { thresholdAmount: '100.00', rewardAmount: '10.00', sort: String(current.length) }])}
            >
              <Plus size={17} />
              添加档位
            </button>
          </div>
          <div className="tier-table">
            <div className="tier-table-head recharge-tier-head">
              <span>累计充值达标</span>
              <span>奖励余额</span>
              <span>操作</span>
            </div>
            <div className="tier-list">
              {tiers.map((tier, index) => (
                <div className="tier-row recharge-tier-row" key={`${tier.id ?? 'new'}-${index}`}>
                  <input
                    type="number"
                    min="0.01"
                    step="0.01"
                    value={tier.thresholdAmount}
                    onChange={(event) => updateTier(index, 'thresholdAmount', event.target.value)}
                  />
                  <input
                    type="number"
                    min="0.01"
                    step="0.01"
                    value={tier.rewardAmount}
                    onChange={(event) => updateTier(index, 'rewardAmount', event.target.value)}
                  />
                  <button type="button" className="icon-btn" disabled={tiers.length <= 1} onClick={() => removeTier(index)}>
                    <Trash2 size={16} />
                  </button>
                </div>
              ))}
            </div>
          </div>
        </div>

        {error && <div className="error-line">{error}</div>}
        <button className="primary-btn wide" type="submit" disabled={saving}>
          <CheckCircle2 size={18} />
          {saving ? '保存中...' : '保存活动'}
        </button>
      </form>
    </div>
  );
}

function CodeModal({ code, onClose, onSaved }: { code: RedeemCode | null; onClose: () => void; onSaved: () => void }) {
  const [form, setForm] = useState<CodePayload>({
    code: code?.code ?? '',
    userId: code?.userId ?? '',
    signDate: code?.signDate ?? '',
    amount: code ? String(code.amount) : '1.00',
    status: code?.status ?? 'AVAILABLE'
  });
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);

  function patch<K extends keyof CodePayload>(key: K, value: CodePayload[K]) {
    setForm((current) => ({ ...current, [key]: value }));
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError('');
    try {
      if (code) {
        await updateCode(code.id, form);
      } else {
        await createCode(form);
      }
      onSaved();
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '保存失败');
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="modal-backdrop">
      <form className="modal" onSubmit={submit}>
        <div className="modal-head">
          <div>
            <span className="eyebrow">{code ? 'Edit Code' : 'New Code'}</span>
            <h2>{code ? '编辑兑换码' : '新增兑换码'}</h2>
          </div>
          <button type="button" className="icon-btn" onClick={onClose} title="关闭">
            <X size={18} />
          </button>
        </div>

        <label>
          兑换码
          <input value={form.code} onChange={(event) => patch('code', event.target.value)} required />
        </label>
        <label>
          用户 ID
          <input value={form.userId ?? ''} onChange={(event) => patch('userId', event.target.value)} placeholder="未绑定时留空" />
        </label>
        <label>
          签到日期
          <input type="date" value={form.signDate ?? ''} onChange={(event) => patch('signDate', event.target.value)} />
        </label>
        <label>
          金额
          <input type="number" min="0.01" step="0.01" value={form.amount} onChange={(event) => patch('amount', event.target.value)} required />
        </label>
        <label>
          状态
          <select value={form.status} onChange={(event) => patch('status', event.target.value as RedeemCodeStatus)}>
            <option value="AVAILABLE">未绑定</option>
            <option value="ASSIGNED">已绑定</option>
            <option value="USED">已使用</option>
            <option value="VOIDED">已作废</option>
          </select>
        </label>

        {error && <div className="error-line">{error}</div>}
        <button className="primary-btn wide" type="submit" disabled={saving}>
          <CheckCircle2 size={18} />
          {saving ? '保存中...' : '保存'}
        </button>
      </form>
    </div>
  );
}

function ImportModal({ onClose, onImported }: { onClose: () => void; onImported: () => void }) {
  const [codesText, setCodesText] = useState('');
  const [amount, setAmount] = useState('1.00');
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError('');
    try {
      const data = await batchImportCodes({ codesText, amount });
      await alertDialog({
        title: '导入完成',
        message: `解析 ${data.totalParsed} 个，成功导入 ${data.imported} 个，重复 ${data.duplicated} 个`,
        confirmText: '知道了'
      });
      onImported();
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '导入失败');
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="modal-backdrop">
      <form className="modal" onSubmit={submit}>
        <div className="modal-head">
          <div>
            <span className="eyebrow">Batch Import</span>
            <h2>批量导入兑换码</h2>
          </div>
          <button type="button" className="icon-btn" onClick={onClose} title="关闭">
            <X size={18} />
          </button>
        </div>

        <label>
          金额
          <input type="number" min="0.01" step="0.01" value={amount} onChange={(event) => setAmount(event.target.value)} required />
        </label>
        <label>
          兑换码
          <textarea
            value={codesText}
            onChange={(event) => setCodesText(event.target.value)}
            rows={8}
            placeholder={'每行一个兑换码，也支持空格、逗号分隔'}
            required
          />
        </label>

        {error && <div className="error-line">{error}</div>}
        <button className="primary-btn wide" type="submit" disabled={saving}>
          <CheckCircle2 size={18} />
          {saving ? '导入中...' : '导入'}
        </button>
      </form>
    </div>
  );
}




