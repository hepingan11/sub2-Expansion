import React, { FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import {
  CalendarCheck2,
  ChevronDown,
  CheckCircle2,
  CircleDollarSign,
  Bookmark,
  BookOpen,
  Code2,
  ExternalLink,
  Globe2,
  GripVertical,
  Home,
  KeyRound,
  LogOut,
  Mail,
  Pencil,
  Plus,
  Search,
  Settings2,
  ShieldCheck,
  ShoppingCart,
  Star,
  Trash2,
  UserRound,
  Wrench,
  X
} from 'lucide-react';
import {
  batchImportCodes,
  bindSocialAccount,
  CheckInResult,
  clearUserSession,
  clearToken,
  CodePayload,
  createCode,
  createFavoriteSite,
  createRechargeActivity,
  claimRechargeReward,
  deleteCode,
  deleteFavoriteSite,
  deleteRechargeActivity,
  fetchCheckInSettings,
  fetchCodes,
  fetchCurrentUser,
  fetchFavoriteSiteGroups,
  fetchFavoriteSites,
  fetchRechargeActivities,
  fetchUserCheckInStatus,
  fetchUserRechargeRewards,
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
  RechargeRewardTier,
  Stats,
  Sub2APISettings,
  Sub2APIUserProfile,
  SocialBindingPayload,
  updateCheckInSettings,
  updateCode,
  updateFavoriteSite,
  updateRechargeActivity,
  UserRechargeRewards,
  userCheckIn,
  userLogin,
  userLogin2FA
} from './api';
import './styles.css';

type DashboardSection = 'home' | 'checkins' | 'favorites' | 'recharge' | 'system';
type AppView = 'login' | 'admin' | 'user';
type LoginMode = 'user' | 'admin';

const emptyStats: Stats = { total: 0, available: 0, assigned: 0, used: 0, voided: 0, amountStats: [] };
const emptySub2APISettings: Sub2APISettings = {
  baseUrl: '',
  authMode: 'password',
  adminApiKey: '',
  adminApiKeySet: false,
  adminEmail: '',
  adminPassword: '',
  adminPasswordSet: false,
  timeoutSeconds: 15
};

const statusText: Record<RedeemCodeStatus, string> = {
  AVAILABLE: '未绑定',
  ASSIGNED: '已绑定',
  USED: '已使用',
  VOIDED: '已作废'
};

const favoriteIconPresets = [
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

const favoriteEmojiPresets = [
  { value: 'preset:rocket', label: '启动', emoji: '🚀' },
  { value: 'preset:fire', label: '热门', emoji: '🔥' },
  { value: 'preset:bulb', label: '灵感', emoji: '💡' },
  { value: 'preset:heart', label: '喜欢', emoji: '❤️' },
  { value: 'preset:money', label: '财务', emoji: '💰' },
  { value: 'preset:chart', label: '数据', emoji: '📈' },
  { value: 'preset:lock', label: '安全', emoji: '🔒' },
  { value: 'preset:gift', label: '福利', emoji: '🎁' }
];

function App() {
  const [view, setView] = useState<AppView>(() => {
    if (getUserToken()) return 'user';
    if (getToken()) return 'admin';
    return 'login';
  });

  if (view === 'admin') {
    return <Dashboard onLogout={() => setView('login')} />;
  }
  if (view === 'user') {
    return <UserDashboard onLogout={() => setView('login')} />;
  }
  return <UnifiedLogin onAdminLogin={() => setView('admin')} onUserLogin={() => setView('user')} />;
}

function UnifiedLogin({ onAdminLogin, onUserLogin }: { onAdminLogin: () => void; onUserLogin: () => void }) {
  const pendingSocialBinding = useMemo(() => getPendingSocialBindingFromURL(), []);
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
        await bindPendingSocialAccount(pendingSocialBinding);
        onUserLogin();
        return;
      }
      setError('登录未返回有效的用户令牌');
    } catch (err) {
      setError(err instanceof Error ? err.message : '登录失败');
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
  const [loading, setLoading] = useState(true);
  const [claimingTierId, setClaimingTierId] = useState<number | null>(null);
  const [checkingIn, setCheckingIn] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState(() => consumeSocialBindingNotice());

  async function loadUser() {
    setLoading(true);
    setError('');
    try {
      const [nextUser, nextRewards, nextCheckInStatus] = await Promise.all([
        fetchCurrentUser(),
        fetchUserRechargeRewards(),
        fetchUserCheckInStatus()
      ]);
      setUser(nextUser);
      setRewards(nextRewards);
      setCheckInStatus(nextCheckInStatus);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载用户信息失败');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadUser();
  }, []);

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
      setSuccess(result.alreadyCheckedIn
        ? '今日已签到'
        : `签到成功，${Number(result.amount).toFixed(2)} 余额已自动入账`);
      await loadUser();
    } catch (err) {
      setError(err instanceof Error ? err.message : '签到失败');
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
      setSuccess(`已领取 ${Number(result.rewardAmount).toFixed(2)} 余额奖励`);
      await loadUser();
    } catch (err) {
      setError(err instanceof Error ? err.message : '领取失败');
    } finally {
      setClaimingTierId(null);
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
      {success && <div className="success-line user-flash">{success}</div>}

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
                  <span>终点 {rewardMaxThreshold.toFixed(2)}</span>
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
                    return (
                      <span
                        className={`reward-progress-marker ${
                          tier.claimed ? 'is-claimed' : tier.eligible ? 'is-ready' : ''
                        }`}
                        key={`${activity.id}-${tier.id}`}
                        style={{ left: `${markerLeft}%` }}
                      />
                    );
                  })}
                </div>
              </div>

              <div className="reward-progress-steps">
                {rewardMilestones.map(({ activity, tier }) => (
                  <div
                    className={`reward-progress-step ${
                      tier.claimed ? 'is-claimed' : tier.eligible ? 'is-ready' : ''
                    }`}
                    key={`${activity.id}-${tier.id}`}
                  >
                    <div>
                      <span className="reward-activity-name">{activity.name}</span>
                      <span>满 {Number(tier.thresholdAmount).toFixed(2)}</span>
                      <strong>奖励 {Number(tier.rewardAmount).toFixed(2)}</strong>
                    </div>
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
                ))}
              </div>
            </article>
          )}
          {!loading && rewardMilestones.length === 0 && (
            <div className="amount-stats-empty">暂无可参与的累计充值活动</div>
          )}
        </div>
      </section>

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
            <dt>角色</dt>
            <dd>{user?.role || '-'}</dd>
          </div>
          <div>
            <dt>运行模式</dt>
            <dd>{user?.run_mode || '-'}</dd>
          </div>
          <div>
            <dt>创建时间</dt>
            <dd>{formatOptionalDate(user?.created_at)}</dd>
          </div>
        </dl>
      </section>
    </main>
  );
}

function Login({ onLogin }: { onLogin: () => void }) {
  const [username, setUsername] = useState('admin');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  async function submit(event: FormEvent) {
    event.preventDefault();
    setLoading(true);
    setError('');
    try {
      await login(username, password);
      onLogin();
    } catch (err) {
      setError(err instanceof Error ? err.message : '登录失败');
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="login-shell">
      <section className="login-panel">
        <div className="brand-mark">
          <ShieldCheck size={26} />
        </div>
        <h1>兑换码管理</h1>
        <p>登录后导入兑换码池，查看签到绑定和兑换状态。</p>
        <form onSubmit={submit} className="login-form">
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
              placeholder=""
            />
          </label>
          {error && <div className="error-line">{error}</div>}
          <button type="submit" disabled={loading}>
            {loading ? '登录中...' : '登录'}
          </button>
        </form>
      </section>
    </main>
  );
}

function Dashboard({ onLogout }: { onLogout: () => void }) {
  const [activeSection, setActiveSection] = useState<DashboardSection>('home');
  const [codes, setCodes] = useState<RedeemCode[]>([]);
  const [stats, setStats] = useState<Stats>(emptyStats);
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
  const [editingRechargeActivity, setEditingRechargeActivity] = useState<RechargeActivity | null>(null);
  const [rechargeModalOpen, setRechargeModalOpen] = useState(false);
  const [dailyMaxUsers, setDailyMaxUsers] = useState(0);
  const [dailyMaxUsersDraft, setDailyMaxUsersDraft] = useState('');
  const [prizeTiers, setPrizeTiers] = useState<PrizeTierSetting[]>([]);
  const [prizeTierDrafts, setPrizeTierDrafts] = useState([{ amount: '1.00', probability: '100.00' }]);
  const [sub2api, setSub2api] = useState<Sub2APISettings>(emptySub2APISettings);
  const [sub2apiDraft, setSub2apiDraft] = useState<Sub2APISettings>(emptySub2APISettings);
  const [settingsSaving, setSettingsSaving] = useState(false);
  const [settingsSaved, setSettingsSaved] = useState(false);

  async function load(nextPage = page) {
    setLoading(true);
    setError('');
    try {
      const [statsData, pageData, settingsData] = await Promise.all([
        fetchStats(),
        fetchCodes({ keyword, status, page: nextPage, size: 10 }),
        fetchCheckInSettings()
      ]);
      setStats(normalizeStats(statsData));
      setCodes(Array.isArray(pageData.content) ? pageData.content : []);
      setPage(pageData.number);
      setTotalPages(Math.max(pageData.totalPages, 1));
      setDailyMaxUsers(settingsData.dailyMaxUsers);
      setDailyMaxUsersDraft(String(settingsData.dailyMaxUsers));
      setPrizeTiers(Array.isArray(settingsData.prizeTiers) ? settingsData.prizeTiers : []);
      setPrizeTierDrafts(toPrizeTierDrafts(settingsData.prizeTiers));
      setSub2api(settingsData.sub2api ?? emptySub2APISettings);
      setSub2apiDraft(toSub2APIDraft(settingsData.sub2api ?? emptySub2APISettings));
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
    } finally {
      setLoading(false);
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
      setError(err instanceof Error ? err.message : '加载收藏网站失败');
    } finally {
      setLoading(false);
    }
  }

  async function loadRechargeActivities() {
    setLoading(true);
    setError('');
    try {
      setRechargeActivities(await fetchRechargeActivities());
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载充值活动失败');
    } finally {
      setLoading(false);
    }
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
  }, [activeSection]);

  const summary = useMemo(() => [
    { label: '总兑换码', value: stats.total, tone: 'ink' },
    { label: '未绑定', value: stats.available, tone: 'green' },
    { label: '已绑定', value: stats.assigned, tone: 'blue' },
    { label: '已使用', value: stats.used, tone: 'blue' },
    { label: '已作废', value: stats.voided, tone: 'red' }
  ], [stats]);
  const amountOptions = useMemo(() => toAmountOptions(stats.amountStats, prizeTierDrafts), [stats.amountStats, prizeTierDrafts]);
  const navItems = [
    { key: 'recharge' as const, label: '充值活动', icon: CircleDollarSign },
    { key: 'home' as const, label: '首页', icon: Home },
    { key: 'checkins' as const, label: '签到管理', icon: CalendarCheck2 },
    { key: 'favorites' as const, label: '网站收藏', icon: Bookmark },
    { key: 'system' as const, label: '系统设置', icon: Settings2 }
  ];

  function logout() {
    clearToken();
    onLogout();
  }

  async function remove(id: number) {
    if (!window.confirm('确认删除这条兑换码记录？')) {
      return;
    }
    await deleteCode(id);
    load(page);
  }

  async function saveCheckInSettings(event: FormEvent) {
    event.preventDefault();
    const nextDailyMaxUsers = Number(dailyMaxUsersDraft);
    if (!Number.isInteger(nextDailyMaxUsers) || nextDailyMaxUsers < 0) {
      setError('每日签到上限必须是大于等于 0 的整数');
      return;
    }

    const parsedPrizeTiers = parsePrizeTierDrafts(prizeTierDrafts);
    if (typeof parsedPrizeTiers === 'string') {
      setError(parsedPrizeTiers);
      return;
    }

    setSettingsSaving(true);
    setSettingsSaved(false);
    setError('');
    try {
      const parsedSub2API = parseSub2APIDraft(sub2apiDraft);
      if (typeof parsedSub2API === 'string') {
        setError(parsedSub2API);
        return;
      }

      const settings = await updateCheckInSettings(nextDailyMaxUsers, parsedPrizeTiers, parsedSub2API);
      setDailyMaxUsers(settings.dailyMaxUsers);
      setDailyMaxUsersDraft(String(settings.dailyMaxUsers));
      setPrizeTiers(settings.prizeTiers);
      setPrizeTierDrafts(toPrizeTierDrafts(settings.prizeTiers));
      setSub2api(settings.sub2api);
      setSub2apiDraft(toSub2APIDraft(settings.sub2api));
      setSettingsSaved(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存设置失败');
    } finally {
      setSettingsSaving(false);
    }
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
      setError(err instanceof Error ? err.message : '保存排序失败');
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
                onClick={() => setActiveSection(item.key)}
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

      {activeSection === 'home' && (
        <>
      <section className="summary-grid">
        {summary.map((item) => (
          <article key={item.label} className={`metric metric-${item.tone}`}>
            <span>{item.label}</span>
            <strong>{item.value}</strong>
          </article>
        ))}
      </section>

      <section className="amount-stats-panel">
        <div className="amount-stats-head">
          <CircleDollarSign size={18} />
          <span>金额库存统计</span>
        </div>
        <div className="amount-stats-grid">
          {stats.amountStats.map((item) => (
            <article className="amount-stat" key={item.amount}>
              <strong>{Number(item.amount).toFixed(2)} 元</strong>
              <div>
                <span>总数</span>
                <b>{item.total}</b>
              </div>
              <div>
                <span>未使用</span>
                <b>{item.available}</b>
              </div>
            </article>
          ))}
          {stats.amountStats.length === 0 && <div className="amount-stats-empty">暂无兑换码库存</div>}
        </div>
      </section>
        </>
      )}

      {activeSection === 'checkins' && (
        <>
      <form className="settings-panel checkin-settings" onSubmit={saveCheckInSettings}>
        <div className="settings-panel-head checkin-settings-head">
          <div className="settings-title">
            <Settings2 size={18} />
            <span>签到设置</span>
          </div>
          <div className="checkin-actions">
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
            <button
              type="button"
              className="ghost-btn"
              onClick={() => {
                setPrizeTierDrafts((current) => [...current, { amount: amountOptions[0] ?? '1.00', probability: '1.00' }]);
                setSettingsSaved(false);
              }}
            >
              <Plus size={17} />
              添加档位
            </button>
            <button className="ghost-btn" type="submit" disabled={settingsSaving || !settingsChanged(dailyMaxUsers, dailyMaxUsersDraft, prizeTiers, prizeTierDrafts, sub2api, sub2apiDraft)}>
              <CheckCircle2 size={17} />
              {settingsSaving ? '保存中...' : '保存'}
            </button>
            {settingsSaved && <span className="settings-saved">已保存</span>}
          </div>
        </div>

        <div className="tier-editor">
          <div className="tier-editor-head">
            <span>兑换码金额概率</span>
            <div className={`probability-total ${prizeTierTotal(prizeTierDrafts) === 100 ? 'is-valid' : ''}`}>
              合计 {prizeTierTotal(prizeTierDrafts).toFixed(2)}%
            </div>
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
                          if (!window.confirm('确认删除这个累计充值活动？')) return;
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
        </>
      )}

      {activeSection === 'system' && (
        <form className="settings-panel" onSubmit={saveCheckInSettings}>
          <div className="settings-panel-head">
            <div className="settings-title">
              <Settings2 size={18} />
              <span>系统设置</span>
            </div>
            <button className="ghost-btn" type="submit" disabled={settingsSaving || !settingsChanged(dailyMaxUsers, dailyMaxUsersDraft, prizeTiers, prizeTierDrafts, sub2api, sub2apiDraft)}>
              <CheckCircle2 size={17} />
              {settingsSaving ? '保存中...' : '保存'}
            </button>
            {settingsSaved && <span className="settings-saved">已保存</span>}
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
      setError(err instanceof Error ? err.message : '保存失败');
    } finally {
      setSaving(false);
    }
  }

  async function removeSite() {
    if (!site || !window.confirm('确认删除这个收藏网站？')) {
      return;
    }
    setDeleting(true);
    setError('');
    try {
      await deleteFavoriteSite(site.id);
      onDeleted();
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除失败');
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
        setError(payload);
        return;
      }
      if (activity) {
        await updateRechargeActivity(activity.id, payload);
      } else {
        await createRechargeActivity(payload);
      }
      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存充值活动失败');
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
      setError(err instanceof Error ? err.message : '保存失败');
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
      window.alert(`解析 ${data.totalParsed} 个，成功导入 ${data.imported} 个，重复 ${data.duplicated} 个`);
      onImported();
    } catch (err) {
      setError(err instanceof Error ? err.message : '导入失败');
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

function formatDateTime(value: string) {
  return value ? value.replace('T', ' ').slice(0, 19) : '-';
}

interface RechargeRewardTierDraft {
  id?: number;
  thresholdAmount: string;
  rewardAmount: string;
  sort: string;
}

function toRechargeTierDrafts(tiers: RechargeRewardTier[] = []): RechargeRewardTierDraft[] {
  const source = tiers.length ? tiers : [{ thresholdAmount: 100, rewardAmount: 10, sort: 0 }];
  return source.map((tier, index) => ({
    id: tier.id,
    thresholdAmount: Number(tier.thresholdAmount).toFixed(2),
    rewardAmount: Number(tier.rewardAmount).toFixed(2),
    sort: String(tier.sort ?? index)
  }));
}

function parseRechargeActivityPayload(input: {
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

function toDateTimeLocal(value?: string | null) {
  if (!value) return '';
  return value.replace(' ', 'T').slice(0, 16);
}

function getPendingSocialBindingFromURL(): SocialBindingPayload | null {
  const params = new URLSearchParams(window.location.search);
  const platform = params.get('platform')?.trim() ?? '';
  const userId = params.get('userid')?.trim() ?? '';
  if (!platform || !userId) {
    return null;
  }
  return { platform, userId };
}

async function bindPendingSocialAccount(binding: SocialBindingPayload | null) {
  if (!binding) {
    return;
  }
  try {
    const result = await bindSocialAccount(binding);
    if (result.bound) {
      sessionStorage.setItem('social_binding_notice', `已绑定 ${result.platform} 账号 ${result.externalUserId}`);
    } else if (result.alreadyBound) {
      sessionStorage.setItem('social_binding_notice', `当前账号已绑定 ${result.platform}，本次不会覆盖`);
    }
  } catch (err) {
    const message = err instanceof Error ? err.message : '社交账号绑定失败';
    sessionStorage.setItem('social_binding_notice', `登录成功，但社交账号绑定失败：${message}`);
  }
}

function consumeSocialBindingNotice() {
  const notice = sessionStorage.getItem('social_binding_notice') ?? '';
  if (notice) {
    sessionStorage.removeItem('social_binding_notice');
  }
  return notice;
}

function formatOptionalDate(value: unknown) {
  return typeof value === 'string' && value ? formatDateTime(value) : '-';
}

function formatToday() {
  return new Date().toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit'
  }).replaceAll('/', '-');
}

function sectionTitle(section: DashboardSection) {
  switch (section) {
    case 'home':
      return '首页';
    case 'checkins':
      return '签到管理';
    case 'favorites':
      return '网站收藏';
    case 'recharge':
      return '充值活动';
    case 'system':
      return '系统设置';
  }
}

function normalizeStats(stats: Stats): Stats {
  return {
    ...emptyStats,
    ...stats,
    amountStats: Array.isArray(stats?.amountStats) ? stats.amountStats : []
  };
}

function toPrizeTierDrafts(prizeTiers: PrizeTierSetting[] = []) {
  const tiers = prizeTiers.length ? prizeTiers : [{ amount: 1, probability: 100 }];
  return tiers.map((tier) => ({
    amount: Number(tier.amount).toFixed(2),
    probability: Number(tier.probability).toFixed(2)
  }));
}

function toSub2APIDraft(settings: Sub2APISettings): Sub2APISettings {
  return {
    ...settings,
    authMode: settings.authMode || 'password',
    adminApiKey: '',
    adminPassword: '',
    timeoutSeconds: settings.timeoutSeconds || 15
  };
}

function updateSub2APIDraft<K extends keyof Sub2APISettings>(
  key: K,
  value: Sub2APISettings[K],
  setSub2apiDraft: React.Dispatch<React.SetStateAction<Sub2APISettings>>,
  setSettingsSaved: React.Dispatch<React.SetStateAction<boolean>>
) {
  setSub2apiDraft((current) => ({ ...current, [key]: value }));
  setSettingsSaved(false);
}

function parseSub2APIDraft(draft: Sub2APISettings): Sub2APISettings | string {
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

function toAmountOptions(amountStats: Stats['amountStats'], drafts: { amount: string; probability: string }[]) {
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

function updatePrizeTierDraft(
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

function parsePrizeTierDrafts(drafts: { amount: string; probability: string }[]): PrizeTierSetting[] | string {
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

function prizeTierTotal(drafts: { amount: string; probability: string }[]) {
  return drafts.reduce((total, tier) => {
    const probability = Number(tier.probability);
    return Number.isFinite(probability) ? total + probability : total;
  }, 0);
}

function settingsChanged(
  dailyMaxUsers: number,
  dailyMaxUsersDraft: string,
  prizeTiers: PrizeTierSetting[],
  prizeTierDrafts: { amount: string; probability: string }[],
  sub2api: Sub2APISettings,
  sub2apiDraft: Sub2APISettings
) {
  if (dailyMaxUsersDraft !== String(dailyMaxUsers)) {
    return true;
  }
  if (JSON.stringify(toPrizeTierDrafts(prizeTiers)) !== JSON.stringify(prizeTierDrafts)) {
    return true;
  }
  return JSON.stringify(toSub2APIDraft(sub2api)) !== JSON.stringify(sub2apiDraft);
}

createRoot(document.getElementById('root')!).render(<App />);

