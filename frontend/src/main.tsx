import React, { FormEvent, useEffect, useMemo, useState } from 'react';
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
  Home,
  KeyRound,
  LogOut,
  Mail,
  Pencil,
  Plus,
  Search,
  Settings2,
  ShieldCheck,
  Star,
  Trash2,
  UserRound,
  Wrench,
  X
} from 'lucide-react';
import {
  batchImportCodes,
  clearToken,
  CodePayload,
  createCode,
  createFavoriteSite,
  deleteCode,
  deleteFavoriteSite,
  fetchCheckInSettings,
  fetchCodes,
  fetchFavoriteSiteGroups,
  fetchFavoriteSites,
  fetchStats,
  FavoriteSite,
  FavoriteSitePayload,
  getToken,
  login,
  PrizeTierSetting,
  RedeemCode,
  RedeemCodeStatus,
  Stats,
  Sub2APISettings,
  updateCheckInSettings,
  updateCode,
  updateFavoriteSite
} from './api';
import './styles.css';

type DashboardSection = 'home' | 'checkins' | 'favorites' | 'system';

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
  { value: 'preset:user', label: '账号', icon: UserRound }
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
  const [authed, setAuthed] = useState(Boolean(getToken()));

  return authed ? <Dashboard onLogout={() => setAuthed(false)} /> : <Login onLogin={() => setAuthed(true)} />;
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
  const [favoriteGroups, setFavoriteGroups] = useState<string[]>([]);
  const [favoriteKeyword, setFavoriteKeyword] = useState('');
  const [favoriteGroup, setFavoriteGroup] = useState('');
  const [favoritePage, setFavoritePage] = useState(0);
  const [favoriteTotalPages, setFavoriteTotalPages] = useState(1);
  const [editingFavorite, setEditingFavorite] = useState<FavoriteSite | null>(null);
  const [favoriteModalOpen, setFavoriteModalOpen] = useState(false);
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
      setStats(statsData);
      setCodes(pageData.content);
      setPage(pageData.number);
      setTotalPages(Math.max(pageData.totalPages, 1));
      setDailyMaxUsers(settingsData.dailyMaxUsers);
      setDailyMaxUsersDraft(String(settingsData.dailyMaxUsers));
      setPrizeTiers(settingsData.prizeTiers);
      setPrizeTierDrafts(toPrizeTierDrafts(settingsData.prizeTiers));
      setSub2api(settingsData.sub2api);
      setSub2apiDraft(toSub2APIDraft(settingsData.sub2api));
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
      setFavoriteGroups(groupsData);
      setFavoritePage(pageData.number);
      setFavoriteTotalPages(Math.max(pageData.totalPages, 1));
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载收藏网站失败');
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

  async function removeFavoriteSite(id: number) {
    if (!window.confirm('确认删除这个收藏网站？')) {
      return;
    }
    await deleteFavoriteSite(id);
    loadFavoriteSites(favoritePage);
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

      <section className="table-panel favorite-table-panel">
        <table>
          <thead>
            <tr>
              <th>图标</th>
              <th>名称</th>
              <th>URL</th>
              <th>简介</th>
              <th>分组</th>
              <th>排序</th>
              <th>创建时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {favoriteSites.map((site) => (
              <tr key={site.id}>
                <td>
                  <SiteIcon site={site} />
                </td>
                <td className="site-name-cell">{site.name}</td>
                <td>
                  <a className="site-url" href={site.url} target="_blank" rel="noreferrer">
                    <ExternalLink size={15} />
                    {site.url}
                  </a>
                </td>
                <td className="site-description-cell">{site.description || '-'}</td>
                <td>{site.group || '-'}</td>
                <td>{site.sort}</td>
                <td>{formatDateTime(site.createdAt)}</td>
                <td>
                  <div className="row-actions">
                    <button title="编辑" onClick={() => { setEditingFavorite(site); setFavoriteModalOpen(true); }}>
                      <Pencil size={16} />
                    </button>
                    <button title="删除" onClick={() => removeFavoriteSite(site.id)}>
                      <Trash2 size={16} />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            {!loading && favoriteSites.length === 0 && (
              <tr>
                <td colSpan={8} className="empty-cell">暂无收藏网站</td>
              </tr>
            )}
            {loading && (
              <tr>
                <td colSpan={8} className="empty-cell">加载中...</td>
              </tr>
            )}
          </tbody>
        </table>
      </section>

      <footer className="pager">
        <button disabled={favoritePage <= 0} onClick={() => loadFavoriteSites(favoritePage - 1)}>上一页</button>
        <span>{favoritePage + 1} / {favoriteTotalPages}</span>
        <button disabled={favoritePage + 1 >= favoriteTotalPages} onClick={() => loadFavoriteSites(favoritePage + 1)}>下一页</button>
      </footer>
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

function FavoriteSiteModal({ site, groups, onClose, onSaved }: { site: FavoriteSite | null; groups: string[]; onClose: () => void; onSaved: () => void }) {
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
        <button className="primary-btn wide" type="submit" disabled={saving}>
          <CheckCircle2 size={18} />
          {saving ? '保存中...' : '保存'}
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

function sectionTitle(section: DashboardSection) {
  switch (section) {
    case 'home':
      return '首页';
    case 'checkins':
      return '签到管理';
    case 'favorites':
      return '网站收藏';
    case 'system':
      return '系统设置';
  }
}

function toPrizeTierDrafts(prizeTiers: PrizeTierSetting[]) {
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
